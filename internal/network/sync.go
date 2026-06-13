package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"termtalk/internal/db"
)

// Frame represents a protocol packet exchanged over TCP.
type Frame struct {
	Type     string      `json:"type"`               // "handshake", "sync_list", "sync_request", "msg"
	UUID     string      `json:"uuid,omitempty"`     // Sender UUID for handshakes
	Username string      `json:"username,omitempty"` // Sender Username for handshakes
	Hashes   []string    `json:"hashes,omitempty"`   // List of message IDs for history sync
	Message  *db.Message `json:"message,omitempty"`  // Single message object
}

// RelayFrame represents the message wrapper used by the relay server.
type RelayFrame struct {
	Type      string          `json:"type"`               // "register", "relay", "msg", "offline", "ping"
	UUID      string          `json:"uuid,omitempty"`     // Client registration UUID
	Username  string          `json:"username,omitempty"` // Client registration Username
	Recipient string          `json:"recipient,omitempty"` // Target Recipient UUID
	Message   json.RawMessage `json:"message,omitempty"`  // Nested Frame payload
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
	localUUID  string
	username   string
	db         *db.Database
	tcpPort    int
	listener   net.Listener
	activeConn map[string]*PeerConnection // Keyed by Peer UUID
	mu         sync.Mutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
	OnMsgRecv  func(msg *db.Message)
	OnPeerSync func(peerUUID string)

	// Relay connection state
	relayAddr   string
	relayConn   net.Conn
	relayEnc    *json.Encoder
	relayDec    *json.Decoder
	relayOnline bool
	relayMu     sync.Mutex
}

// NewSyncManager creates a SyncManager instance.
func NewSyncManager(localUUID, username string, tcpPort int, database *db.Database) *SyncManager {
	return &SyncManager{
		localUUID:  localUUID,
		username:   username,
		db:         database,
		tcpPort:    tcpPort,
		relayAddr:  "localhost:55558", // Default fallback
		activeConn: make(map[string]*PeerConnection),
		stopChan:   make(chan struct{}),
	}
}

// SetRelayAddr sets the relay server address dynamically.
func (sm *SyncManager) SetRelayAddr(addr string) {
	sm.relayMu.Lock()
	defer sm.relayMu.Unlock()
	sm.relayAddr = addr
}

// UpdateCredentials updates the profile credentials thread-safely after registration.
func (sm *SyncManager) UpdateCredentials(uuid, username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.localUUID = uuid
	sm.username = username
}

// Start starts the TCP server listening for incoming connections and the relay connection loop.
func (sm *SyncManager) Start() error {
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
	go sm.relayLoop()

	return nil
}

// Stop closes the TCP listener and all active connections.
func (sm *SyncManager) Stop() {
	close(sm.stopChan)
	if sm.listener != nil {
		sm.listener.Close()
	}

	sm.mu.Lock()
	for _, pc := range sm.activeConn {
		pc.Close()
	}
	sm.mu.Unlock()

	sm.relayMu.Lock()
	if sm.relayConn != nil {
		sm.relayConn.Close()
	}
	sm.relayMu.Unlock()

	sm.wg.Wait()
}

// ConnectToPeer initiates a TCP connection to a peer, performs the handshake, and syncs history.
func (sm *SyncManager) ConnectToPeer(c *db.Contact) error {
	sm.mu.Lock()
	if _, active := sm.activeConn[c.UUID]; active {
		sm.mu.Unlock()
		return nil // Already connected
	}
	sm.mu.Unlock()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.IP, c.Port), 3*time.Second)
	if err != nil {
		return err
	}

	pc := NewPeerConnection(conn)

	// Perform Handshake
	err = pc.Send(Frame{
		Type:     "handshake",
		UUID:     sm.localUUID,
		Username: sm.username,
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

	pc.UUID = resp.UUID
	pc.Username = resp.Username

	// Track connection
	sm.mu.Lock()
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
	msg := &db.Message{
		Sender:    sm.localUUID,
		Recipient: peerUUID,
		Content:   content,
		Timestamp: time.Now(),
		Status:    "queued",
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
				_ = sm.db.UpdateMessageStatus(msg.ID, "synced")
				msg.Status = "synced"
				if sm.OnMsgRecv != nil {
					sm.OnMsgRecv(msg)
				}
			}
		}()
	} else {
		// Attempt routing through the relay server
		go func() {
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
	history, err := sm.db.GetChatHistory(sm.localUUID, pc.UUID)
	if err != nil {
		return
	}

	hashes := make([]string, len(history))
	for i, m := range history {
		hashes[i] = m.ID
	}

	// Send list of hashes we have
	_ = pc.Send(Frame{
		Type:   "sync_list",
		Hashes: hashes,
	})
}

// sendRelayFrame encapsulates and sends a Frame to the relay server.
func (sm *SyncManager) sendRelayFrame(recipientUUID string, f Frame) error {
	sm.relayMu.Lock()
	enc := sm.relayEnc
	online := sm.relayOnline
	sm.relayMu.Unlock()

	if !online || enc == nil {
		return fmt.Errorf("relay offline")
	}

	payload, err := json.Marshal(f)
	if err != nil {
		return err
	}

	return enc.Encode(RelayFrame{
		Type:      "relay",
		Recipient: recipientUUID,
		Message:   payload,
	})
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
	err := pc.Send(Frame{
		Type:     "handshake",
		UUID:     sm.localUUID,
		Username: sm.username,
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
			sm.handleSyncList(pc.UUID, frame.Hashes, func(f Frame) { _ = pc.Send(f) })
		case "sync_request":
			sm.handleSyncRequest(pc.UUID, frame.Hashes, func(f Frame) { _ = pc.Send(f) })
		case "msg":
			sm.handleIncomingMessage(pc.UUID, frame.Message)
		}
	}
}

// handleSyncList processes the message log comparison.
func (sm *SyncManager) handleSyncList(peerUUID string, peerHashes []string, sendFunc func(Frame)) {
	history, err := sm.db.GetChatHistory(sm.localUUID, peerUUID)
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
		if !peerHashesMap[m.ID] && m.Status == "synced" {
			sendFunc(Frame{
				Type:    "msg",
				Message: &m,
			})
		}
	}
}

// handleSyncRequest processes requests for missing messages.
func (sm *SyncManager) handleSyncRequest(peerUUID string, requestedHashes []string, sendFunc func(Frame)) {
	history, err := sm.db.GetChatHistory(sm.localUUID, peerUUID)
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
	if m.Status == "ack" {
		_ = sm.db.UpdateMessageStatus(m.Content, "synced")
		m.Status = "synced"
		if sm.OnMsgRecv != nil {
			sm.OnMsgRecv(m)
		}
		return
	}

	// Validate message integrity
	if m.GenerateID() != m.ID {
		return
	}

	m.Status = "synced"
	if err := sm.db.SaveMessage(m); err != nil {
		return
	}

	if m.Recipient == sm.localUUID && m.Sender == peerUUID {
		if sm.OnMsgRecv != nil {
			sm.OnMsgRecv(m)
		}

		// Send delivery confirmation ACK back
		ackMsg := &db.Message{
			Sender:    sm.localUUID,
			Recipient: peerUUID,
			Content:   m.ID,
			Timestamp: time.Now(),
			Status:    "ack",
		}
		ackMsg.ID = ackMsg.GenerateID()

		sm.mu.Lock()
		pc, localOnline := sm.activeConn[peerUUID]
		sm.mu.Unlock()

		if localOnline {
			_ = pc.Send(Frame{
				Type:    "msg",
				Message: ackMsg,
			})
		} else {
			_ = sm.sendRelayFrame(peerUUID, Frame{
				Type:    "msg",
				Message: ackMsg,
			})
		}
	}
}

// relayLoop connects to and processes incoming frames from the relay server.
func (sm *SyncManager) relayLoop() {
	defer sm.wg.Done()

	for {
		select {
		case <-sm.stopChan:
			return
		default:
			sm.relayMu.Lock()
			localUUID := sm.localUUID
			username := sm.username
			addr := sm.relayAddr
			sm.relayMu.Unlock()

			if localUUID == "" {
				time.Sleep(1 * time.Second)
				continue
			}

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			enc := json.NewEncoder(conn)
			dec := json.NewDecoder(conn)

			// Send registration
			reg := RelayFrame{
				Type:     "register",
				UUID:     localUUID,
				Username: username,
			}
			if err := enc.Encode(reg); err != nil {
				conn.Close()
				time.Sleep(2 * time.Second)
				continue
			}

			var ack RelayFrame
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

			// Trigger automatic history synchronization for all known contacts
			go sm.triggerRelaySyncAll()

			errChan := make(chan error, 1)
			go func() {
				for {
					var frame RelayFrame
					if err := dec.Decode(&frame); err != nil {
						errChan <- err
						return
					}

					if frame.Type == "msg" {
						var inner Frame
						if err := json.Unmarshal(frame.Message, &inner); err == nil {
							sm.handleRelayFrame(frame.UUID, inner)
						}
					}
				}
			}()

			select {
			case <-sm.stopChan:
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
		_ = sm.sendRelayFrame(senderUUID, f)
	}

	switch inner.Type {
	case "sync_list":
		sm.handleSyncList(senderUUID, inner.Hashes, sendFunc)
	case "sync_request":
		sm.handleSyncRequest(senderUUID, inner.Hashes, sendFunc)
	case "msg":
		sm.handleIncomingMessage(senderUUID, inner.Message)
	}
}

// triggerRelaySyncAll triggers background sync request frames for all stored contacts.
func (sm *SyncManager) triggerRelaySyncAll() {
	time.Sleep(1 * time.Second) // Let registration settle
	contacts, err := sm.db.ListContacts()
	if err != nil {
		return
	}

	for _, c := range contacts {
		history, err := sm.db.GetChatHistory(sm.localUUID, c.UUID)
		if err != nil {
			continue
		}

		hashes := make([]string, len(history))
		for i, m := range history {
			hashes[i] = m.ID
		}

		_ = sm.sendRelayFrame(c.UUID, Frame{
			Type:   "sync_list",
			Hashes: hashes,
		})
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
