package network

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"termtalk/internal/crypto"
	"termtalk/internal/db"
	"termtalk/internal/protocol"
)

// Frame represents a protocol packet exchanged over TCP.
type Frame struct {
	Type      string          `json:"type"`                 // "handshake", "sync_list", "sync_request", "msg", "ice_offer", "ice_answer", "typing"
	UUID      string          `json:"uuid,omitempty"`       // Sender UUID for handshakes
	Username  string          `json:"username,omitempty"`   // Sender Username for handshakes
	Hashes    []string        `json:"hashes,omitempty"`     // List of message IDs for history sync
	Message   *db.Message     `json:"message,omitempty"`    // Single message object
	ICESignal json.RawMessage `json:"ice_signal,omitempty"` // ICE negotiation data (ICESignal struct)
}

// PeerConnection wraps an active TCP connection to a peer.
type PeerConnection struct {
	UUID     string
	Username string
	conn     net.Conn
	enc      *json.Encoder
	dec      *json.Decoder
	mu       sync.Mutex
}

// NewPeerConnection initializes a PeerConnection.
func NewPeerConnection(conn net.Conn) *PeerConnection {
	return &PeerConnection{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
}

// Send sends a Frame to the peer safely.
func (pc *PeerConnection) Send(f Frame) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.enc.Encode(f)
}

// Close closes the connection.
func (pc *PeerConnection) Close() error {
	return pc.conn.Close()
}

// SyncManager manages TCP socket listening, outgoing connections, and message sync.
type SyncManager struct {
	localUUID       string
	username        string
	publicKey       []byte
	privateKey      []byte
	x25519PublicKey []byte
	db              *db.Database
	tcpPort         int
	listener        net.Listener
	activeConn      map[string]*PeerConnection // Keyed by Peer UUID
	mu              sync.Mutex
	stopChan        chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
	OnMsgRecv       func(msg *db.Message)
	OnPeerSync      func(peerUUID string)

	// ICE NAT hole punching manager
	iceManager *ICEManager

	// Relay connection state
	relayAddr   string
	relayConn   net.Conn
	relayEnc    *json.Encoder
	relayDec    *json.Decoder
	relayOnline bool
	relayMu     sync.Mutex

	// Relay event callbacks (set by Client before Start)
	OnSearchResult func(users []protocol.UserInfo)
	OnOnlineList   func(users []protocol.UserInfo)
	OnReadAck      func(senderUUID string, messageIDs []string)
	OnUserList     func(users []protocol.UserInfo)
	OnTyping       func(senderUUID string)
}

// DefaultRelayAddr is the public TermTalk relay node hosted on Fly.io
const DefaultRelayAddr = "termtalk-relay.fly.dev:55558"

// NewSyncManager creates a SyncManager instance.
func NewSyncManager(localUUID, username string, tcpPort int, database *db.Database) *SyncManager {
	sm := &SyncManager{
		localUUID:  localUUID,
		username:   username,
		db:         database,
		tcpPort:    tcpPort,
		relayAddr:  DefaultRelayAddr,
		activeConn: make(map[string]*PeerConnection),
		stopChan:   make(chan struct{}),
	}
	sm.iceManager = NewICEManager(sm)
	return sm
}

// SetRelayAddr sets the relay server address dynamically.
func (sm *SyncManager) SetRelayAddr(addr string) {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	sm.relayAddr = addr
}

// UpdateCredentials updates the profile credentials thread-safely after registration.
func (sm *SyncManager) UpdateCredentials(uuid, username string, publicKey, privateKey, x25519PublicKey []byte) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.localUUID = uuid
	sm.username = username
	sm.publicKey = publicKey
	sm.privateKey = privateKey
	sm.x25519PublicKey = x25519PublicKey
}

// getLocalUUID safely reads localUUID under lock.
func (sm *SyncManager) getLocalUUID() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.localUUID
}

// getUsername safely reads username under lock.
func (sm *SyncManager) getUsername() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.username
}

// Start starts the TCP server listening for incoming connections and the relay connection loop.
func (sm *SyncManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	l, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", sm.tcpPort))
	if err == nil {
		sm.listener = l
		sm.wg.Add(1)
		go sm.acceptLoop()
	} else {
		log.Printf("Warning: local TCP listener could not start (port occupied?): %v", err)
	}

	sm.wg.Add(1)
	go sm.relayLoop(ctx)

	return nil
}

// Stop closes the TCP listener and all active connections.
func (sm *SyncManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.stopChan)

		// Shut down ICE manager first
		if sm.iceManager != nil {
			sm.iceManager.Close()
		}

		sm.mu.Lock()
		if sm.listener != nil {
			sm.listener.Close()
		}
		for _, pc := range sm.activeConn {
			pc.Close()
		}
		sm.mu.Unlock()

		sm.relayMu.Lock()
		if sm.relayConn != nil {
			sm.relayConn.Close()
		}
		sm.relayMu.Unlock()
	})

	sm.wg.Wait()
}

// ConnectToPeer initiates a TCP connection to a peer, performs the handshake, and syncs history.
func (sm *SyncManager) ConnectToPeer(ctx context.Context, c *db.Contact) error {
	sm.mu.Lock()
	if _, active := sm.activeConn[c.UUID]; active {
		sm.mu.Unlock()
		return nil // Already connected
	}
	sm.mu.Unlock()

	// Use context with a 3-second dial timeout
	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	defer dialCancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", fmt.Sprintf("%s:%d", c.IP, c.Port))
	if err != nil {
		return err
	}

	// Set handshake deadline from context, fallback to 5-second deadline
	handshakeCtx, hsCancel := context.WithTimeout(ctx, 5*time.Second)
	defer hsCancel()
	if deadline, ok := handshakeCtx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	pc := NewPeerConnection(conn)

	// Perform Handshake
	localUUID := sm.getLocalUUID()
	username := sm.getUsername()
	err = pc.Send(Frame{
		Type:     "handshake",
		UUID:     localUUID,
		Username: username,
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake write failed: %w", err)
	}

	var resp Frame
	if err := pc.dec.Decode(&resp); err != nil || resp.Type != "handshake" {
		conn.Close()
		return fmt.Errorf("handshake response failed: %w", err)
	}

	// Reset connection deadline after successful handshake
	conn.SetDeadline(time.Time{})

	pc.UUID = resp.UUID
	pc.Username = resp.Username

	// Track connection, closing duplicate active connections if found
	sm.mu.Lock()
	if oldPc, exists := sm.activeConn[pc.UUID]; exists {
		oldPc.Close()
	}
	sm.activeConn[pc.UUID] = pc
	sm.mu.Unlock()

	sm.wg.Add(1)
	go sm.handleConnection(pc)

	// Trigger synchronization history
	go sm.SyncHistory(pc)

	return nil
}

// SendMessage sends a message to a peer. If peer is online locally, it transmits via TCP. Otherwise, it fallbacks to the relay server.
func (sm *SyncManager) SendMessage(peerUUID string, content string) error {
	localUUID := sm.getLocalUUID()
	msg := &db.Message{
		Sender:    localUUID,
		Recipient: peerUUID,
		Content:   content,
		Timestamp: time.Now(),
		Status:    string(db.StatusQueued),
	}
	msg.ID = msg.GenerateID()

	// Save to DB as queued (outbox)
	if err := sm.db.SaveMessage(msg); err != nil {
		return err
	}

	sm.mu.Lock()
	pc, localOnline := sm.activeConn[peerUUID]
	sm.mu.Unlock()

	if localOnline {
		go func() {
			err := pc.Send(Frame{
				Type:    "msg",
				Message: msg,
			})
			if err == nil {
				// Mark as synced locally
				if err := sm.db.UpdateMessageStatus(msg.ID, string(db.StatusSynced)); err != nil {
					log.Printf("sync: failed to update message status to synced: %v", err)
				}
				msg.Status = string(db.StatusSynced)
				if sm.OnMsgRecv != nil {
					sm.OnMsgRecv(msg)
				}
			}
		}()
	} else {
		// Attempt routing through the relay server
		go func() {
			// Check if message will be encrypted (contact has X25519 key)
			sm.mu.Lock()
			privKey := sm.privateKey
			sm.mu.Unlock()
			if len(privKey) > 0 {
				recipientContact, cErr := sm.db.GetContact(peerUUID)
				if cErr == nil && recipientContact != nil && len(recipientContact.X25519PublicKey) == 32 {
					msg.Encrypted = true
				}
			}

			err := sm.sendRelayFrame(peerUUID, Frame{
				Type:    "msg",
				Message: msg,
			})
			if err == nil {
				// Note: message remains queued until recipient client returns delivery ACK
				if sm.OnMsgRecv != nil {
					sm.OnMsgRecv(msg)
				}
			}
		}()
	}

	return nil
}

// SyncHistory exchanges message hashes and syncs history.
func (sm *SyncManager) SyncHistory(pc *PeerConnection) {
	localUUID := sm.getLocalUUID()
	history, err := sm.db.GetChatHistory(localUUID, pc.UUID)
	if err != nil {
		return
	}

	hashes := make([]string, len(history))
	for i, m := range history {
		hashes[i] = m.ID
	}

	// Send list of hashes we have
	if err := pc.Send(Frame{
		Type:   "sync_list",
		Hashes: hashes,
	}); err != nil {
		log.Printf("sync: failed to send sync_list: %v", err)
	}
}

// sendRelayFrame encapsulates and sends a Frame to the relay server.
// The relayMu is held for the entire Encode() call to prevent concurrent writes (CE-002).
// If private key is available, the payload is signed with Ed25519.
func (sm *SyncManager) sendRelayFrame(recipientUUID string, f Frame) error {
	payload, err := json.Marshal(f)
	if err != nil {
		return err
	}

	relayFrame := protocol.RelayFrame{
		Type:      "relay",
		Recipient: recipientUUID,
		Message:   payload,
	}

	// Sign the payload with Ed25519 if keys are available
	sm.mu.Lock()
	privKey := sm.privateKey
	pubKey := sm.publicKey
	x25519Pub := sm.x25519PublicKey
	sm.mu.Unlock()

	if len(privKey) > 0 && len(pubKey) > 0 {
		p := &db.Profile{PrivateKey: privKey}
		relayFrame.Signature = base64.StdEncoding.EncodeToString(p.Sign(payload))
		relayFrame.PublicKey = base64.StdEncoding.EncodeToString(pubKey)
	}

	// Include sender's X25519 public key so the recipient can decrypt
	if len(x25519Pub) > 0 {
		relayFrame.X25519PublicKey = base64.StdEncoding.EncodeToString(x25519Pub)
	}

	// Encrypt the payload with NaCl box if we have both our private key
	// and the recipient's X25519 public key
	if len(privKey) > 0 {
		recipientContact, err := sm.db.GetContact(recipientUUID)
		if err == nil && recipientContact != nil && len(recipientContact.X25519PublicKey) == 32 {
			var recipientX25519Pub [32]byte
			copy(recipientX25519Pub[:], recipientContact.X25519PublicKey)

			ciphertext, nonce, encErr := crypto.Encrypt(payload, ed25519.PrivateKey(privKey), recipientX25519Pub)
			if encErr == nil {
				// Replace plaintext payload with encrypted payload
				relayFrame.Message = json.RawMessage(`"` + ciphertext + `"`)
				relayFrame.Nonce = nonce
				relayFrame.Encrypted = true
				// Re-sign the encrypted payload
				if len(pubKey) > 0 {
					p := &db.Profile{PrivateKey: privKey}
					relayFrame.Signature = base64.StdEncoding.EncodeToString(p.Sign(relayFrame.Message))
				}
			} else {
				log.Printf("sync: encryption failed for %s, sending plaintext: %v", recipientUUID, encErr)
			}
		}
	}

	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()

	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}

	return sm.relayEnc.Encode(relayFrame)
}

// acceptLoop accepts incoming local TCP connections.
func (sm *SyncManager) acceptLoop() {
	defer sm.wg.Done()

	for {
		conn, err := sm.listener.Accept()
		if err != nil {
			select {
			case <-sm.stopChan:
				return
			default:
				time.Sleep(1 * time.Second)
				continue
			}
		}

		go sm.handleIncomingHandshake(conn)
	}
}

func (sm *SyncManager) handleIncomingHandshake(conn net.Conn) {
	pc := NewPeerConnection(conn)

	// Read handshake
	var frame Frame
	if err := pc.dec.Decode(&frame); err != nil || frame.Type != "handshake" {
		conn.Close()
		return
	}

	pc.UUID = frame.UUID
	pc.Username = frame.Username

	// Respond to Handshake
	localUUID := sm.getLocalUUID()
	username := sm.getUsername()
	err := pc.Send(Frame{
		Type:     "handshake",
		UUID:     localUUID,
		Username: username,
	})
	if err != nil {
		conn.Close()
		return
	}

	// Track connection
	sm.mu.Lock()
	if oldPc, exists := sm.activeConn[pc.UUID]; exists {
		oldPc.Close()
	}
	sm.activeConn[pc.UUID] = pc
	sm.mu.Unlock()

	sm.wg.Add(1)
	go sm.handleConnection(pc)

	// Trigger synchronization history
	go sm.SyncHistory(pc)
}

// handleConnection handles reading frames from a direct peer.
func (sm *SyncManager) handleConnection(pc *PeerConnection) {
	defer sm.wg.Done()
	defer func() {
		pc.Close()
		sm.mu.Lock()
		delete(sm.activeConn, pc.UUID)
		sm.mu.Unlock()
	}()

	for {
		var frame Frame
		err := pc.dec.Decode(&frame)
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}

		switch frame.Type {
		case "sync_list":
			sm.handleSyncList(pc.UUID, frame.Hashes, func(f Frame) {
				if err := pc.Send(f); err != nil {
					log.Printf("sync: failed to send frame to peer %s: %v", pc.UUID, err)
				}
			})
		case "sync_request":
			sm.handleSyncRequest(pc.UUID, frame.Hashes, func(f Frame) {
				if err := pc.Send(f); err != nil {
					log.Printf("sync: failed to send frame to peer %s: %v", pc.UUID, err)
				}
			})
		case "msg":
			sm.handleIncomingMessage(pc.UUID, frame.Message)
		}
	}
}

// handleSyncList processes the message log comparison.
func (sm *SyncManager) handleSyncList(peerUUID string, peerHashes []string, sendFunc func(Frame)) {
	localUUID := sm.getLocalUUID()
	history, err := sm.db.GetChatHistory(localUUID, peerUUID)
	if err != nil {
		return
	}

	ourHashesMap := make(map[string]bool)
	for _, m := range history {
		ourHashesMap[m.ID] = true
	}

	var missingFromUs []string
	for _, h := range peerHashes {
		if !ourHashesMap[h] {
			missingFromUs = append(missingFromUs, h)
		}
	}

	// Request messages we don't have
	if len(missingFromUs) > 0 {
		sendFunc(Frame{
			Type:   "sync_request",
			Hashes: missingFromUs,
		})
	}

	// Send messages we have that the peer doesn't
	peerHashesMap := make(map[string]bool)
	for _, h := range peerHashes {
		peerHashesMap[h] = true
	}

	for _, m := range history {
		if !peerHashesMap[m.ID] && m.Status == string(db.StatusSynced) {
			sendFunc(Frame{
				Type:    "msg",
				Message: &m,
			})
		}
	}
}

// handleSyncRequest processes requests for missing messages.
func (sm *SyncManager) handleSyncRequest(peerUUID string, requestedHashes []string, sendFunc func(Frame)) {
	localUUID := sm.getLocalUUID()
	history, err := sm.db.GetChatHistory(localUUID, peerUUID)
	if err != nil {
		return
	}

	for _, id := range requestedHashes {
		for _, m := range history {
			if m.ID == id {
				sendFunc(Frame{
					Type:    "msg",
					Message: &m,
				})
				break
			}
		}
	}
}

// handleIncomingMessage processes incoming messages. If it's a delivery ACK, it completes the sync.
func (sm *SyncManager) handleIncomingMessage(peerUUID string, m *db.Message) {
	if m == nil {
		return
	}

	// If it is a delivery ACK, complete the message status sync
	if m.Status == string(db.StatusAck) {
		if err := sm.db.UpdateMessageStatus(m.Content, string(db.StatusSynced)); err != nil {
			log.Printf("sync: failed to update ack'd message status: %v", err)
		}
		m.Status = string(db.StatusSynced)
		if sm.OnMsgRecv != nil {
			sm.OnMsgRecv(m)
		}
		return
	}

	// Validate message integrity
	if m.GenerateID() != m.ID {
		return
	}

	m.Status = string(db.StatusSynced)
	if err := sm.db.SaveMessage(m); err != nil {
		return
	}

	localUUID := sm.getLocalUUID()
	if m.Recipient == localUUID && m.Sender == peerUUID {
		if sm.OnMsgRecv != nil {
			sm.OnMsgRecv(m)
		}

		// Send delivery confirmation ACK back
		ackMsg := &db.Message{
			Sender:    localUUID,
			Recipient: peerUUID,
			Content:   m.ID,
			Timestamp: time.Now(),
			Status:    string(db.StatusAck),
		}
		ackMsg.ID = ackMsg.GenerateID()

		sm.mu.Lock()
		pc, localOnline := sm.activeConn[peerUUID]
		sm.mu.Unlock()

		if localOnline {
			if err := pc.Send(Frame{
				Type:    "msg",
				Message: ackMsg,
			}); err != nil {
				log.Printf("sync: failed to send ACK to peer %s: %v", peerUUID, err)
			}
		} else {
			if err := sm.sendRelayFrame(peerUUID, Frame{
				Type:    "msg",
				Message: ackMsg,
			}); err != nil {
				log.Printf("sync: failed to send ACK via relay to peer %s: %v", peerUUID, err)
			}
		}
	}
}

// relayLoop connects to and processes incoming frames from the relay server.
func (sm *SyncManager) relayLoop(ctx context.Context) {
	defer sm.wg.Done()

	for {
		select {
		case <-sm.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			localUUID := sm.getLocalUUID()
			username := sm.getUsername()
			sm.relayMu.Lock()
			addr := sm.relayAddr
			sm.relayMu.Unlock()

			if localUUID == "" {
				time.Sleep(1 * time.Second)
				continue
			}

			var d net.Dialer
			dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
			conn, err := d.DialContext(dialCtx, "tcp", addr)
			dialCancel()
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			enc := json.NewEncoder(conn)
			dec := json.NewDecoder(conn)

			// Send registration
			reg := protocol.RelayFrame{
				Type:     "register",
				UUID:     localUUID,
				Username: username,
			}
			sm.mu.Lock()
			if len(sm.publicKey) > 0 {
				reg.PublicKey = base64.StdEncoding.EncodeToString(sm.publicKey)
			}
			if len(sm.x25519PublicKey) > 0 {
				reg.X25519PublicKey = base64.StdEncoding.EncodeToString(sm.x25519PublicKey)
			}
			sm.mu.Unlock()
			if err := enc.Encode(reg); err != nil {
				conn.Close()
				time.Sleep(2 * time.Second)
				continue
			}

			var ack protocol.RelayFrame
			if err := dec.Decode(&ack); err != nil || ack.Type != "registered" {
				conn.Close()
				time.Sleep(2 * time.Second)
				continue
			}

			sm.relayMu.Lock()
			sm.relayConn = conn
			sm.relayEnc = enc
			sm.relayDec = dec
			sm.relayOnline = true
			sm.relayMu.Unlock()

			log.Printf("Successfully registered on TermTalk Relay: %s", addr)

			// Drain locally queued messages through the relay
			go sm.drainOutbox()

			errChan := make(chan error, 1)
			pingStop := make(chan struct{})
			var closeOnce sync.Once
			closePing := func() {
				closeOnce.Do(func() {
					close(pingStop)
				})
			}

			// Relay Keepalive Heartbeat loop
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						sm.relayMu.Lock()
						if sm.relayEnc != nil {
							if err := sm.relayEnc.Encode(protocol.RelayFrame{Type: "ping"}); err != nil {
								log.Printf("sync: failed to send relay ping: %v", err)
							}
						}
						sm.relayMu.Unlock()
					case <-pingStop:
						return
					}
				}
			}()

			go func() {
				for {
					var frame protocol.RelayFrame
					if err := dec.Decode(&frame); err != nil {
						errChan <- err
						closePing()
						return
					}

					switch frame.Type {
					case "msg":
						msgPayload := frame.Message
						wasEncrypted := false

						// Decrypt NaCl box if encrypted
						if frame.Encrypted && frame.Nonce != "" && frame.X25519PublicKey != "" {
							sm.mu.Lock()
							privKey := sm.privateKey
							sm.mu.Unlock()

							if len(privKey) > 0 {
								senderX25519Bytes, x25519Err := base64.StdEncoding.DecodeString(frame.X25519PublicKey)
								if x25519Err == nil && len(senderX25519Bytes) == 32 {
									var senderX25519Pub [32]byte
									copy(senderX25519Pub[:], senderX25519Bytes)

									// The encrypted payload is a JSON string (base64 ciphertext)
									var ciphertextB64 string
									if err := json.Unmarshal(msgPayload, &ciphertextB64); err == nil {
										plaintext, decErr := crypto.Decrypt(ciphertextB64, frame.Nonce, ed25519.PrivateKey(privKey), senderX25519Pub)
										if decErr == nil {
											msgPayload = plaintext
											wasEncrypted = true
										} else {
											log.Printf("sync: decryption failed from %s: %v", frame.UUID, decErr)
										}
									}
								}
							}
						}

						var inner Frame
						if err := json.Unmarshal(msgPayload, &inner); err == nil {
							// Verify Ed25519 signature if present (warn-only in v0.4.0)
							if !frame.Encrypted && frame.Signature != "" && frame.PublicKey != "" {
								pubKeyBytes, pkErr := base64.StdEncoding.DecodeString(frame.PublicKey)
								sigBytes, sigErr := base64.StdEncoding.DecodeString(frame.Signature)
								if pkErr != nil || sigErr != nil {
									log.Printf("sync: invalid base64 in signature/public_key from %s", frame.UUID)
								} else if !db.Verify(pubKeyBytes, frame.Message, sigBytes) {
									log.Printf("sync: WARNING: invalid Ed25519 signature from %s", frame.UUID)
								}
							}

							// Store sender's X25519 public key for future encryption
							if frame.X25519PublicKey != "" {
								x25519Bytes, x25519Err := base64.StdEncoding.DecodeString(frame.X25519PublicKey)
								if x25519Err == nil && len(x25519Bytes) == 32 {
									contact, cErr := sm.db.GetContact(frame.UUID)
									if cErr == nil && contact != nil && len(contact.X25519PublicKey) == 0 {
										contact.X25519PublicKey = x25519Bytes
										_ = sm.db.UpsertContact(contact)
									}
								}
							}

							// Mark inner message as encrypted if decryption succeeded
							if wasEncrypted && inner.Message != nil {
								inner.Message.Encrypted = true
							}

							sm.handleRelayFrame(frame.UUID, inner)
						}
					case "stored":
						if frame.MessageID != "" {
							if err := sm.db.UpdateMessageStatus(frame.MessageID, string(db.StatusStored)); err != nil {
								log.Printf("sync: failed to update message %s to stored: %v", frame.MessageID, err)
							}
						}
					case "delivered":
						if frame.MessageID != "" {
							if err := sm.db.UpdateMessageStatus(frame.MessageID, string(db.StatusSynced)); err != nil {
								log.Printf("sync: failed to update message %s to synced: %v", frame.MessageID, err)
							}
						}
					case "search_result":
						if sm.OnSearchResult != nil {
							sm.OnSearchResult(frame.Users)
						}
					case "online_list":
						if sm.OnOnlineList != nil {
							sm.OnOnlineList(frame.Users)
						}
					case "user_list":
						if sm.OnUserList != nil {
							sm.OnUserList(frame.Users)
						}
					case "read_ack":
						if sm.OnReadAck != nil {
							sm.OnReadAck(frame.UUID, frame.MessageIDs)
						}
					case "delete":
						if frame.MessageIDs != nil {
							if err := sm.db.DeleteMessages(frame.MessageIDs); err != nil {
								log.Printf("sync: failed to delete messages: %v", err)
							}
							if sm.OnMsgRecv != nil {
								// Trigger UI refresh by sending a nil message event
								sm.OnMsgRecv(nil)
							}
						}
					}
				}
			}()

			select {
			case <-sm.stopChan:
				closePing()
				conn.Close()
				return
			case <-errChan:
				conn.Close()
				sm.relayMu.Lock()
				sm.relayConn = nil
				sm.relayEnc = nil
				sm.relayDec = nil
				sm.relayOnline = false
				sm.relayMu.Unlock()
				time.Sleep(2 * time.Second)
			}
		}
	}
}

// handleRelayFrame routes received relay frames to the respective protocol handler.
func (sm *SyncManager) handleRelayFrame(senderUUID string, inner Frame) {
	sendFunc := func(f Frame) {
		if err := sm.sendRelayFrame(senderUUID, f); err != nil {
			log.Printf("sync: failed to send relay frame to %s: %v", senderUUID, err)
		}
	}

	switch inner.Type {
	case "sync_list":
		sm.handleSyncList(senderUUID, inner.Hashes, sendFunc)
	case "sync_request":
		sm.handleSyncRequest(senderUUID, inner.Hashes, sendFunc)
	case "msg":
		sm.handleIncomingMessage(senderUUID, inner.Message)
	case "ice_offer":
		if sm.iceManager != nil {
			signal, err := parseICESignal(inner)
			if err != nil {
				log.Printf("ice: failed to parse offer from %s: %v", senderUUID, err)
				return
			}
			sm.iceManager.HandleOffer(senderUUID, signal)
		}
	case "ice_answer":
		if sm.iceManager != nil {
			signal, err := parseICESignal(inner)
			if err != nil {
				log.Printf("ice: failed to parse answer from %s: %v", senderUUID, err)
				return
			}
			sm.iceManager.HandleAnswer(senderUUID, signal)
		}
	case "typing":
		if sm.OnTyping != nil {
			sm.OnTyping(senderUUID)
		}
	}
}

// triggerRelaySyncAll triggers background sync request frames for all stored contacts.
func (sm *SyncManager) triggerRelaySyncAll() {
	time.Sleep(1 * time.Second) // Let registration settle
	contacts, err := sm.db.ListContacts()
	if err != nil {
		return
	}

	localUUID := sm.getLocalUUID()
	for _, c := range contacts {
		history, err := sm.db.GetChatHistory(localUUID, c.UUID)
		if err != nil {
			continue
		}

		hashes := make([]string, len(history))
		for i, m := range history {
			hashes[i] = m.ID
		}

		if err := sm.sendRelayFrame(c.UUID, Frame{
			Type:   "sync_list",
			Hashes: hashes,
		}); err != nil {
			log.Printf("sync: failed to send relay sync_list to %s: %v", c.UUID, err)
		}
	}
}

// IsPeerOnline checks if a peer is connected directly.
func (sm *SyncManager) IsPeerOnline(peerUUID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, online := sm.activeConn[peerUUID]
	return online
}

// IsRelayOnline checks if the client is connected to the relay server.
func (sm *SyncManager) IsRelayOnline() bool {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	return sm.relayOnline
}

// AttemptICEConnection triggers an ICE NAT hole punching attempt to a peer.
// If ICE succeeds, the connection is registered in activeConn and used for
// direct messaging. If ICE fails, relay messaging continues as fallback.
func (sm *SyncManager) AttemptICEConnection(peerUUID string) {
	if sm.iceManager == nil {
		return
	}

	// Don't attempt ICE if we already have a direct connection
	sm.mu.Lock()
	_, hasConn := sm.activeConn[peerUUID]
	sm.mu.Unlock()
	if hasConn {
		return
	}

	sm.iceManager.InitiateConnection(peerUUID)
}

// ListenerPort returns the actual TCP port the listener is bound to.
// Useful when started with port 0 (OS-assigned) in tests.
func (sm *SyncManager) ListenerPort() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.listener == nil {
		return 0
	}
	return sm.listener.Addr().(*net.TCPAddr).Port
}

// SendSearchRequest sends a search query to the relay server.
func (sm *SyncManager) SendSearchRequest(query string) error {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()

	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}

	return sm.relayEnc.Encode(protocol.RelayFrame{
		Type:  "search",
		Query: query,
	})
}

// SendWhoOnline requests the list of online users from the relay server.
func (sm *SyncManager) SendWhoOnline() error {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()

	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}

	return sm.relayEnc.Encode(protocol.RelayFrame{
		Type: "who_online",
	})
}

// SendListUsers requests the full user directory from the relay server.
// CE-005: relayMu is held for the entire Encode() call.
func (sm *SyncManager) SendListUsers() error {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}
	return sm.relayEnc.Encode(protocol.RelayFrame{Type: "list_users"})
}

// SendReadAck sends a batch read receipt to the relay for forwarding to the original sender.
// CE-005: relayMu is held for the entire Encode() call.
func (sm *SyncManager) SendReadAck(recipientUUID string, messageIDs []string) error {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}
	return sm.relayEnc.Encode(protocol.RelayFrame{
		Type:       "read_ack",
		Recipient:  recipientUUID,
		MessageIDs: messageIDs,
	})
}

// SendDeleteRequest sends a delete frame to the recipient via relay.
// CE-005: relayMu is held for the entire Encode() call.
func (sm *SyncManager) SendDeleteRequest(recipientUUID string, messageIDs []string) error {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	if !sm.relayOnline || sm.relayEnc == nil {
		return fmt.Errorf("relay offline")
	}
	return sm.relayEnc.Encode(protocol.RelayFrame{
		Type:       "delete",
		Recipient:  recipientUUID,
		MessageIDs: messageIDs,
	})
}

// SendTyping sends an ephemeral typing indicator frame to a peer via the relay.
func (sm *SyncManager) SendTyping(recipientUUID string) error {
	return sm.sendRelayFrame(recipientUUID, Frame{
		Type: "typing",
	})
}

// drainOutbox re-sends all locally queued messages through the relay after connecting.
func (sm *SyncManager) drainOutbox() {
	time.Sleep(500 * time.Millisecond) // Let registration settle

	localUUID := sm.getLocalUUID()
	if localUUID == "" {
		return
	}

	msgs, err := sm.db.GetQueuedMessages(localUUID)
	if err != nil {
		log.Printf("sync: drainOutbox: failed to query queued messages: %v", err)
		return
	}

	if len(msgs) == 0 {
		return
	}

	log.Printf("sync: draining outbox: %d queued messages", len(msgs))

	for _, msg := range msgs {
		if err := sm.sendRelayFrame(msg.Recipient, Frame{
			Type:    "msg",
			Message: &msg,
		}); err != nil {
			log.Printf("sync: drainOutbox: failed to relay message %s: %v", msg.ID[:8], err)
		}
	}
}
