package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"termtalk/internal/db"
)

// Frame represents a protocol packet exchanged over TCP.
type Frame struct {
	Type     string      `json:"type"`                // "handshake", "sync_list", "sync_request", "msg"
	UUID     string      `json:"uuid,omitempty"`      // Sender UUID for handshakes
	Username string      `json:"username,omitempty"`  // Sender Username for handshakes
	Hashes   []string    `json:"hashes,omitempty"`    // List of message IDs for history sync
	Message  *db.Message `json:"message,omitempty"`   // Single message object
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
}

// NewSyncManager creates a SyncManager instance.
func NewSyncManager(localUUID, username string, tcpPort int, database *db.Database) *SyncManager {
	return &SyncManager{
		localUUID:  localUUID,
		username:   username,
		db:         database,
		tcpPort:    tcpPort,
		activeConn: make(map[string]*PeerConnection),
		stopChan:   make(chan struct{}),
	}
}

// UpdateCredentials updates the profile credentials thread-safely after registration.
func (sm *SyncManager) UpdateCredentials(uuid, username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.localUUID = uuid
	sm.username = username
}

// Start starts the TCP server listening for incoming connections.
func (sm *SyncManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	l, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", sm.tcpPort))
	if err != nil {
		return fmt.Errorf("failed to start TCP listener on port %d: %w", sm.tcpPort, err)
	}
	sm.listener = l

	sm.wg.Add(1)
	go sm.acceptLoop()

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

// SendMessage sends a message to a peer. If peer is online, it transmits over TCP immediately.
func (sm *SyncManager) SendMessage(peerUUID string, content string) error {
	msg := &db.Message{
		Sender:    sm.localUUID,
		Recipient: peerUUID,
		Content:   content,
		Timestamp: time.Now(),
		Status:    "queued",
	}
	msg.ID = msg.GenerateID()

	// Save to DB as queued
	if err := sm.db.SaveMessage(msg); err != nil {
		return err
	}

	sm.mu.Lock()
	pc, online := sm.activeConn[peerUUID]
	sm.mu.Unlock()

	if online {
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

// acceptLoop accepts incoming TCP connections.
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
	// Close any existing connection to this peer first
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

// handleConnection handles reading frames from a peer.
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
				return // Graceful close
			}
			return // Connection error
		}

		switch frame.Type {
		case "sync_list":
			sm.handleSyncList(pc, frame.Hashes)
		case "sync_request":
			sm.handleSyncRequest(pc, frame.Hashes)
		case "msg":
			sm.handleIncomingMessage(pc, frame.Message)
		}
	}
}

func (sm *SyncManager) handleSyncList(pc *PeerConnection, peerHashes []string) {
	// 1. Check which of the peer's hashes we don't have
	history, err := sm.db.GetChatHistory(sm.localUUID, pc.UUID)
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

	// 2. Request the messages we don't have
	if len(missingFromUs) > 0 {
		_ = pc.Send(Frame{
			Type:   "sync_request",
			Hashes: missingFromUs,
		})
	}

	// 3. Send the messages we have that the peer doesn't
	peerHashesMap := make(map[string]bool)
	for _, h := range peerHashes {
		peerHashesMap[h] = true
	}

	for _, m := range history {
		if !peerHashesMap[m.ID] {
			// Peer is missing this message. Send it.
			_ = pc.Send(Frame{
				Type:    "msg",
				Message: &m,
			})
		}
	}
}

func (sm *SyncManager) handleSyncRequest(pc *PeerConnection, requestedHashes []string) {
	for _, id := range requestedHashes {
		// Fetch from db and send
		rows, err := sm.db.GetChatHistory(sm.localUUID, pc.UUID)
		if err != nil {
			continue
		}
		for _, m := range rows {
			if m.ID == id {
				_ = pc.Send(Frame{
					Type:    "msg",
					Message: &m,
				})
				break
			}
		}
	}
}

func (sm *SyncManager) handleIncomingMessage(pc *PeerConnection, m *db.Message) {
	if m == nil {
		return
	}

	// Verify message hash matches content to ensure integrity
	if m.GenerateID() != m.ID {
		return // Hash mismatch, reject
	}

	// Force status to synced since it is successfully received
	m.Status = "synced"

	// Save to DB
	if err := sm.db.SaveMessage(m); err != nil {
		return
	}

	// Send confirmation back if this was a new, direct message sent to us (not from history sync)
	// We can check if it's sent to us and we haven't already marked it as synced
	if m.Recipient == sm.localUUID && m.Sender == pc.UUID {
		// Trigger UI callback
		if sm.OnMsgRecv != nil {
			sm.OnMsgRecv(m)
		}
	}
}

// IsPeerOnline checks if a peer is connected.
func (sm *SyncManager) IsPeerOnline(peerUUID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, online := sm.activeConn[peerUUID]
	return online
}
