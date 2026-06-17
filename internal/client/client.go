package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"termtalk/internal/db"
	"termtalk/internal/network"
	"termtalk/internal/protocol"
)

// Event is a domain event emitted by the Client and consumed by the TUI.
type Event interface{ isEvent() }

// PeerDiscoveredEvent is fired when a new Peer is found on the local network.
type PeerDiscoveredEvent struct{ Contact *db.Contact }

func (PeerDiscoveredEvent) isEvent() {}

// MessageReceivedEvent is fired when an incoming Message arrives over TCP.
type MessageReceivedEvent struct{ Message *db.Message }

func (MessageReceivedEvent) isEvent() {}

// SearchResultEvent is fired when search results arrive from the relay.
type SearchResultEvent struct{ Users []protocol.UserInfo }

func (SearchResultEvent) isEvent() {}

// OnlineListEvent is fired when the online user list arrives from the relay.
type OnlineListEvent struct{ Users []protocol.UserInfo }

func (OnlineListEvent) isEvent() {}

// ReadAckEvent is fired when a read receipt arrives from a contact.
type ReadAckEvent struct {
	SenderUUID string
	MessageIDs []string
}

func (ReadAckEvent) isEvent() {}

// Client is the unified coordinator for the TermTalk application.
// It encapsulates the Database, PeerDiscovery daemon, and SyncManager
// behind a single, testable interface so the TUI never touches
// infrastructure directly.
type Client struct {
	db        *db.Database
	discovery *network.PeerDiscovery
	syncMgr   *network.SyncManager
	profile   *db.Profile
	events    chan Event
	mu        sync.RWMutex
	tcpPort   int
}

// New opens (or creates) the SQLite database at dbPath, initialises the
// PeerDiscovery daemon and SyncManager, and returns a ready-to-use Client.
// Networking is NOT started until Register is called (first boot) or
// Start is called explicitly (subsequent boots).
func New(dbPath string, tcpPort int) (*Client, error) {
	database, err := db.NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("client: failed to open database: %w", err)
	}

	events := make(chan Event, 100)

	c := &Client{
		db:      database,
		events:  events,
		tcpPort: tcpPort,
	}

	// Build skeleton managers; credentials filled in after registration.
	c.syncMgr = network.NewSyncManager("", "", tcpPort, database)
	c.discovery = network.NewPeerDiscovery("", "", tcpPort, database)

	// Wire network callbacks into the event channel.
	c.discovery.OnPeerFound = func(contact *db.Contact) {
		c.events <- PeerDiscoveredEvent{Contact: contact}
	}
	c.syncMgr.OnMsgRecv = func(msg *db.Message) {
		c.events <- MessageReceivedEvent{Message: msg}
	}
	c.syncMgr.OnSearchResult = func(users []protocol.UserInfo) {
		c.events <- SearchResultEvent{Users: users}
	}
	c.syncMgr.OnOnlineList = func(users []protocol.UserInfo) {
		c.events <- OnlineListEvent{Users: users}
	}
	c.syncMgr.OnReadAck = func(senderUUID string, messageIDs []string) {
		c.events <- ReadAckEvent{SenderUUID: senderUUID, MessageIDs: messageIDs}
	}

	return c, nil
}

// SetRelayAddr updates the relay server address for the SyncManager.
func (c *Client) SetRelayAddr(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncMgr.SetRelayAddr(addr)
}

// GetProfile returns the locally cached profile, or nil if not yet registered.
func (c *Client) GetProfile() *db.Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profile
}

// LoadProfile reads the profile from the database and caches it in the Client.
// Returns (nil, nil) when no profile exists yet (first-boot).
func (c *Client) LoadProfile() (*db.Profile, error) {
	p, err := c.db.GetProfile()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.profile = p
	c.mu.Unlock()
	return p, nil
}

// Register saves a new profile with the given username, updates credentials
// on the networking daemons, and starts them. Returns an error if a profile
// already exists or if the username is empty.
func (c *Client) Register(username string) (*db.Profile, error) {
	if username == "" {
		return nil, fmt.Errorf("client: username must not be empty")
	}
	c.mu.Lock()
	if c.profile != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("client: profile already registered")
	}
	c.mu.Unlock()

	p, err := db.NewProfile(username)
	if err != nil {
		return nil, fmt.Errorf("client: failed to generate profile: %w", err)
	}
	if err := c.db.SaveProfile(p); err != nil {
		return nil, fmt.Errorf("client: failed to save profile: %w", err)
	}

	c.mu.Lock()
	c.profile = p
	c.mu.Unlock()

	c.syncMgr.UpdateCredentials(p.UUID, p.Username, p.PublicKey, p.PrivateKey)
	c.discovery.UpdateCredentials(p.UUID, p.Username)

	return p, nil
}

// Start starts the network daemons using the already-loaded profile credentials.
// Call this on subsequent boots (profile already exists in DB).
func (c *Client) Start(ctx context.Context) error {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()

	if p == nil {
		return fmt.Errorf("client: cannot start networking without a registered profile")
	}

	c.syncMgr.UpdateCredentials(p.UUID, p.Username, p.PublicKey, p.PrivateKey)
	c.discovery.UpdateCredentials(p.UUID, p.Username)

	if err := c.syncMgr.Start(ctx); err != nil {
		return fmt.Errorf("client: TCP sync server: %w", err)
	}
	if err := c.discovery.Start(ctx); err != nil {
		c.syncMgr.Stop()
		return fmt.Errorf("client: UDP discovery: %w", err)
	}

	return nil
}

// Stop gracefully shuts down all network daemons and closes the database.
func (c *Client) Stop() {
	c.syncMgr.Stop()
	c.discovery.Stop()
	c.db.Close()
}

// GetPublicKeyBase64 returns the base64-encoded Ed25519 public key, or empty if not set.
func (c *Client) GetPublicKeyBase64() string {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()
	if p == nil || len(p.PublicKey) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(p.PublicKey)
}

// Events returns the read-only channel of domain events (PeerDiscoveredEvent,
// MessageReceivedEvent). The TUI should listen on this channel.
func (c *Client) Events() <-chan Event {
	return c.events
}

// ListContacts returns all Contacts stored in the local database.
func (c *Client) ListContacts() ([]db.Contact, error) {
	return c.db.ListContacts()
}

// AddContact manually registers a Contact (for offline / sneakernet peers).
func (c *Client) AddContact(username, uuid string) error {
	if username == "" || uuid == "" {
		return fmt.Errorf("client: username and uuid must not be empty")
	}
	return c.db.UpsertContact(&db.Contact{
		UUID:     uuid,
		Username: username,
		IP:       "offline",
		Port:     0,
		LastSeen: time.Now(),
	})
}

// DeleteContact removes a contact from the local database.
// Message history is preserved for archival purposes.
func (c *Client) DeleteContact(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("client: uuid must not be empty")
	}
	return c.db.DeleteContact(uuid)
}

// SendMessage sends a Message to peerUUID. The message is saved locally with
// status "queued"; if the Peer is online it is delivered over TCP immediately.
func (c *Client) SendMessage(peerUUID, content string) error {
	return c.syncMgr.SendMessage(peerUUID, content)
}

// GetChatHistory returns all Messages exchanged between the local Peer and
// the Contact identified by contactUUID, ordered by timestamp ascending.
func (c *Client) GetChatHistory(contactUUID string) ([]db.Message, error) {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()

	if p == nil {
		return nil, fmt.Errorf("client: no local profile loaded")
	}
	return c.db.GetChatHistory(p.UUID, contactUUID)
}

// ExportSync writes an Outbox sync file for the given contact to exportPath.
func (c *Client) ExportSync(contactUUID, exportPath string) error {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()

	if p == nil {
		return fmt.Errorf("client: no local profile loaded")
	}
	return c.db.ExportSyncFile(contactUUID, p, exportPath)
}

// ImportSync reads a sync file from importPath, merges its Messages into the
// local database, and registers the sender as a Contact.
func (c *Client) ImportSync(importPath string) (*db.SyncFile, error) {
	return c.db.ImportSyncFile(importPath)
}

// IsPeerOnline reports whether a direct TCP connection to the given peer UUID
// is currently established.
func (c *Client) IsPeerOnline(peerUUID string) bool {
	return c.syncMgr.IsPeerOnline(peerUUID)
}

// ConnectToPeer dials a discovered Peer over TCP and performs history sync.
func (c *Client) ConnectToPeer(ctx context.Context, contact *db.Contact) error {
	return c.syncMgr.ConnectToPeer(ctx, contact)
}

// SearchUsers sends a search query to the relay server.
// Results arrive asynchronously as a SearchResultEvent on the Events channel.
func (c *Client) SearchUsers(query string) error {
	return c.syncMgr.SendSearchRequest(query)
}

// GetOnlineUsers requests the list of currently online users from the relay.
// Results arrive asynchronously as an OnlineListEvent on the Events channel.
func (c *Client) GetOnlineUsers() error {
	return c.syncMgr.SendWhoOnline()
}

// SendReadAck sends a batch read receipt to the message sender via the relay.
func (c *Client) SendReadAck(contactUUID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return c.syncMgr.SendReadAck(contactUUID, messageIDs)
}

// GetUnreadCount returns the count of unread messages from a contact.
func (c *Client) GetUnreadCount(contactUUID string) (int, error) {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()
	if p == nil {
		return 0, fmt.Errorf("client: no local profile loaded")
	}
	return c.db.GetUnreadCount(p.UUID, contactUUID)
}

// MarkMessagesRead marks a batch of messages as read in the database.
func (c *Client) MarkMessagesRead(messageIDs []string) error {
	return c.db.MarkMessagesRead(messageIDs)
}
