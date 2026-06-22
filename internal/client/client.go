package client

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nod/internal/db"
	"nod/internal/network"
	"nod/internal/protocol"
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

// UserListEvent is fired when the full user directory arrives from the relay.
type UserListEvent struct{ Users []protocol.UserInfo }

func (UserListEvent) isEvent() {}

// TypingEvent is fired when a typing indicator arrives from a contact.
type TypingEvent struct{ SenderUUID string }

func (TypingEvent) isEvent() {}

// ICEConnectedEvent is fired when an ICE connection succeeds or falls back to relay.
type ICEConnectedEvent struct {
	PeerUUID string
	Direct   bool
}

func (ICEConnectedEvent) isEvent() {}

// ReadAckEvent is fired when a read receipt arrives from a contact.
type ReadAckEvent struct {
	SenderUUID string
	MessageIDs []string
}

func (ReadAckEvent) isEvent() {}

// ReactionEvent is fired when a reaction is received.
type ReactionEvent struct{ Reaction *db.Reaction }

func (ReactionEvent) isEvent() {}

// Client is the unified coordinator for the Nod application.
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
	c.syncMgr.OnUserList = func(users []protocol.UserInfo) {
		c.events <- UserListEvent{Users: users}
	}
	c.syncMgr.OnTyping = func(senderUUID string) {
		c.events <- TypingEvent{SenderUUID: senderUUID}
	}
	c.syncMgr.OnReaction = func(reaction *db.Reaction) {
		c.events <- ReactionEvent{Reaction: reaction}
	}
	c.syncMgr.OnICEStatus = func(peerUUID string, direct bool) {
		c.events <- ICEConnectedEvent{PeerUUID: peerUUID, Direct: direct}
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

	c.syncMgr.UpdateCredentials(p.UUID, p.Username, p.PublicKey, p.PrivateKey, p.X25519PublicKey)
	c.discovery.UpdateCredentials(p.UUID, p.Username)

	return p, nil
}

// ChangeUsername validates and updates the user's display name, persists it
// to the database, refreshes the in-memory profile cache, and pushes the
// new credentials to the networking layer.
func (c *Client) ChangeUsername(newUsername string) error {
	if newUsername == "" {
		return fmt.Errorf("client: username must not be empty")
	}
	if len(newUsername) > 32 {
		return fmt.Errorf("client: username must be 32 characters or fewer")
	}

	if err := c.db.UpdateUsername(newUsername); err != nil {
		return fmt.Errorf("client: failed to update username: %w", err)
	}

	c.mu.Lock()
	c.profile.Username = newUsername
	p := c.profile
	c.mu.Unlock()

	c.syncMgr.UpdateCredentials(p.UUID, newUsername, p.PublicKey, p.PrivateKey, p.X25519PublicKey)
	c.discovery.UpdateCredentials(p.UUID, newUsername)

	return nil
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

	c.syncMgr.UpdateCredentials(p.UUID, p.Username, p.PublicKey, p.PrivateKey, p.X25519PublicKey)
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

// GetContact returns a single contact by UUID.
func (c *Client) GetContact(uuid string) (*db.Contact, error) {
	return c.db.GetContact(uuid)
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
func (c *Client) SendMessage(peerUUID, content string, replyTo string) error {
	return c.syncMgr.SendMessage(peerUUID, content, replyTo)
}

// GetMessage retrieves a single message by its ID.
func (c *Client) GetMessage(id string) (*db.Message, error) {
	return c.db.GetMessage(id)
}

// GetChatHistory returns Messages exchanged between the local Peer and
// the Contact identified by contactUUID, ordered by timestamp ascending.
// Supports pagination via limit and offset. Pass 0, 0 for defaults.
func (c *Client) GetChatHistory(contactUUID string, limit, offset int) ([]db.Message, error) {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()

	if p == nil {
		return nil, fmt.Errorf("client: no local profile loaded")
	}
	return c.db.GetChatHistory(p.UUID, contactUUID, limit, offset)
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

// AttemptICEConnection triggers an ICE NAT hole punching attempt to a peer.
// If ICE succeeds, the direct connection is used for messaging.
// If ICE fails, relay messaging continues as fallback. Non-blocking.
func (c *Client) AttemptICEConnection(peerUUID string) {
	c.syncMgr.AttemptICEConnection(peerUUID)
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

// ListUsers requests the full user directory from the relay.
// Results arrive asynchronously as a UserListEvent on the Events channel.
func (c *Client) ListUsers() error {
	return c.syncMgr.SendListUsers()
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

// SetContactVerified sets the verified flag for a contact.
func (c *Client) SetContactVerified(uuid string, verified bool) error {
	return c.db.SetContactVerified(uuid, verified)
}

// SendTyping sends an ephemeral typing indicator to a contact via the relay.
func (c *Client) SendTyping(recipientUUID string) error {
	return c.syncMgr.SendTyping(recipientUUID)
}

// DeleteMessagesLocal removes messages from local DB only (delete for me).
func (c *Client) DeleteMessagesLocal(messageIDs []string) error {
	return c.db.DeleteMessages(messageIDs)
}

// DeleteMessagesForEveryone sends a delete frame via relay and removes locally.
func (c *Client) DeleteMessagesForEveryone(contactUUID string, messageIDs []string) error {
	if err := c.syncMgr.SendDeleteRequest(contactUUID, messageIDs); err != nil {
		return err
	}
	return c.db.DeleteMessages(messageIDs)
}

// EditMessageContent updates a message's content and marks it as edited.
func (c *Client) EditMessageContent(id string, newContent string) error {
	return c.db.UpdateMessageContent(id, newContent)
}

// SendReaction sends (or toggles off) an emoji reaction on a message.
func (c *Client) SendReaction(recipientUUID, messageID, emoji string) error {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("client: no local profile loaded")
	}

	id := db.GenerateReactionID(messageID, p.UUID, emoji)
	ts := time.Now().UTC().Format(time.RFC3339)

	// Toggle: check if reaction already exists
	existing, err := c.db.GetReactions(messageID)
	if err != nil {
		return err
	}
	for _, r := range existing {
		if r.SenderUUID == p.UUID && r.Emoji == emoji {
			// Already reacted with same emoji - remove it (toggle)
			return c.RemoveReaction(recipientUUID, messageID, emoji)
		}
	}

	reaction := &db.Reaction{
		ID:         id,
		MessageID:  messageID,
		SenderUUID: p.UUID,
		Emoji:      emoji,
		Timestamp:  ts,
	}

	if err := c.db.AddReaction(id, messageID, p.UUID, emoji, ts); err != nil {
		return err
	}

	// Send to peer via relay
	go func() {
		_ = c.syncMgr.SendReactionFrame(recipientUUID, reaction)
	}()

	return nil
}

// RemoveReaction removes a reaction and notifies the peer.
func (c *Client) RemoveReaction(recipientUUID, messageID, emoji string) error {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()
	if p == nil {
		return fmt.Errorf("client: no local profile loaded")
	}

	if err := c.db.RemoveReaction(messageID, p.UUID, emoji); err != nil {
		return err
	}

	// Send a removal frame (empty timestamp signals removal)
	id := db.GenerateReactionID(messageID, p.UUID, emoji)
	reaction := &db.Reaction{
		ID:         id,
		MessageID:  messageID,
		SenderUUID: p.UUID,
		Emoji:      emoji,
		Timestamp:  "",
	}
	go func() {
		_ = c.syncMgr.SendReactionFrame(recipientUUID, reaction)
	}()

	return nil
}

// GetChatReactions returns reactions for all messages in a chat.
func (c *Client) GetChatReactions(contactUUID string) (map[string][]db.Reaction, error) {
	c.mu.RLock()
	p := c.profile
	c.mu.RUnlock()
	if p == nil {
		return nil, fmt.Errorf("client: no local profile loaded")
	}
	return c.db.GetChatReactions(p.UUID, contactUUID)
}

// GetNotificationsEnabled returns true if desktop notifications are enabled.
// Defaults to true if the setting has not been configured.
func (c *Client) GetNotificationsEnabled() bool {
	val, err := c.db.GetSetting("notifications_enabled")
	if err != nil || val == "" {
		return true // default: notifications on
	}
	return val == "true"
}

// SetNotificationsEnabled persists the notification toggle preference.
func (c *Client) SetNotificationsEnabled(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return c.db.SetSetting("notifications_enabled", val)
}

// BlockContact sets the blocked flag for a contact.
func (c *Client) BlockContact(uuid string) error {
	return c.db.BlockContact(uuid)
}

// UnblockContact clears the blocked flag for a contact.
func (c *Client) UnblockContact(uuid string) error {
	return c.db.UnblockContact(uuid)
}

// PinContact sets the pinned flag for a contact.
func (c *Client) PinContact(uuid string) error {
	return c.db.PinContact(uuid)
}

// UnpinContact clears the pinned flag for a contact.
func (c *Client) UnpinContact(uuid string) error {
	return c.db.UnpinContact(uuid)
}

// ArchiveContact sets the archived flag for a contact.
func (c *Client) ArchiveContact(uuid string) error {
	return c.db.ArchiveContact(uuid)
}

// UnarchiveContact clears the archived flag for a contact.
func (c *Client) UnarchiveContact(uuid string) error {
	return c.db.UnarchiveContact(uuid)
}

// ── File Attachment Methods (F11/F12: File & Image Sharing) ──

// UploadsDir returns the path to the uploads directory, creating it if needed.
func (c *Client) UploadsDir() (string, error) {
	dir := filepath.Join(filepath.Dir(c.db.Path()), "uploads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// SaveUpload stores file data to disk and returns an Attachment with content-addressed ID.
func (c *Client) SaveUpload(filename string, data []byte) (*db.Attachment, error) {
	dir, err := c.UploadsDir()
	if err != nil {
		return nil, err
	}

	// Content-addressed ID
	hash := sha256.Sum256(data)
	id := fmt.Sprintf("%x", hash)

	// Determine extension from filename
	ext := filepath.Ext(filename)
	storedName := id + ext

	// Write file
	path := filepath.Join(dir, storedName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}

	// Detect MIME type from extension
	mimeType := "application/octet-stream"
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".svg":
		mimeType = "image/svg+xml"
	case ".pdf":
		mimeType = "application/pdf"
	case ".txt":
		mimeType = "text/plain"
	case ".mp3":
		mimeType = "audio/mpeg"
	case ".mp4":
		mimeType = "video/mp4"
	case ".zip":
		mimeType = "application/zip"
	}

	att := &db.Attachment{
		ID:       id,
		Filename: filename,
		MIMEType: mimeType,
		Size:     int64(len(data)),
	}
	return att, nil
}

// GetUploadPath resolves the on-disk path for an attachment by its content-addressed ID.
func (c *Client) GetUploadPath(attachmentID string) (string, error) {
	dir, err := c.UploadsDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		nameNoExt := strings.TrimSuffix(name, filepath.Ext(name))
		if nameNoExt == attachmentID {
			return filepath.Join(dir, name), nil
		}
	}
	return "", fmt.Errorf("attachment not found: %s", attachmentID)
}

// SaveAttachment delegates to the database.
func (c *Client) SaveAttachment(a *db.Attachment) error {
	return c.db.SaveAttachment(a)
}

// GetAttachmentsByMsgID delegates to the database.
func (c *Client) GetAttachmentsByMsgID(msgID string) ([]db.Attachment, error) {
	return c.db.GetAttachmentsByMsgID(msgID)
}

// GetAttachmentsForMessages delegates to the database.
func (c *Client) GetAttachmentsForMessages(msgIDs []string) (map[string][]db.Attachment, error) {
	return c.db.GetAttachmentsForMessages(msgIDs)
}

// SearchMessages searches all messages by content.
func (c *Client) SearchMessages(query string, limit int) ([]db.Message, error) {
	return c.db.SearchMessages(query, limit)
}
