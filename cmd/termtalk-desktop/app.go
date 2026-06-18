package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"termtalk/internal/client"
	"termtalk/internal/db"
	"termtalk/internal/ui"
)

// App struct holds the desktop application state and Wails bindings.
type App struct {
	ctx         context.Context
	client      *client.Client
	dbPath      string
	dataDir     string
	onlineUsers map[string]bool // relay-reported online users
	onlineMu    sync.RWMutex
}

// NewApp creates a new App binding struct.
func NewApp(dbPath, dataDir string) *App {
	return &App{
		dbPath:      dbPath,
		dataDir:     dataDir,
		onlineUsers: make(map[string]bool),
	}
}

// startup is called by Wails when the app starts. Initializes the Client facade.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize the client (opens DB, creates network managers)
	c, err := client.New(a.dbPath, 0) // port 0 = auto-assign
	if err != nil {
		log.Printf("desktop: client init error: %v", err)
		return
	}
	a.client = c

	// Load or prompt for registration
	profile, err := c.LoadProfile()
	if err != nil {
		log.Printf("desktop: no profile found, registration needed")
		return
	}

	// Start networking (UDP discovery + TCP sync + relay)
	if err := c.Start(ctx); err != nil {
		log.Printf("desktop: failed to start networking: %v", err)
	}

	// Consume events in background and emit to React frontend
	go a.eventLoop(profile)
}

// eventLoop reads events from the Client channel and emits them to the frontend.
func (a *App) eventLoop(profile *db.Profile) {
	for event := range a.client.Events() {
		switch e := event.(type) {
		case client.MessageReceivedEvent:
			wailsruntime.EventsEmit(a.ctx, "new_message", map[string]interface{}{
				"sender":  e.Message.Sender,
				"content": e.Message.Content,
			})
			// A message arriving proves the sender is reachable
			a.onlineMu.Lock()
			a.onlineUsers[e.Message.Sender] = true
			a.onlineMu.Unlock()
			wailsruntime.EventsEmit(a.ctx, "contacts_changed")
		case client.TypingEvent:
			wailsruntime.EventsEmit(a.ctx, "typing", map[string]interface{}{
				"sender": e.SenderUUID,
			})
			// Typing also proves the sender is reachable
			a.onlineMu.Lock()
			a.onlineUsers[e.SenderUUID] = true
			a.onlineMu.Unlock()
		case client.PeerDiscoveredEvent:
			wailsruntime.EventsEmit(a.ctx, "peer_discovered", map[string]interface{}{
				"uuid":     e.Contact.UUID,
				"username": e.Contact.Username,
			})
			wailsruntime.EventsEmit(a.ctx, "contacts_changed")
		case client.ReactionEvent:
			wailsruntime.EventsEmit(a.ctx, "reaction", map[string]interface{}{
				"messageId": e.Reaction.MessageID,
				"emoji":     e.Reaction.Emoji,
			})
		case client.ReadAckEvent:
			wailsruntime.EventsEmit(a.ctx, "read_ack", map[string]interface{}{
				"sender":     e.SenderUUID,
				"messageIds": e.MessageIDs,
			})
		case client.OnlineListEvent:
			// Update local online users map for presence checks
			a.onlineMu.Lock()
			a.onlineUsers = make(map[string]bool)
			var users []map[string]string
			for _, u := range e.Users {
				a.onlineUsers[u.UUID] = true
				users = append(users, map[string]string{
					"uuid":     u.UUID,
					"username": u.Username,
				})
			}
			a.onlineMu.Unlock()
			wailsruntime.EventsEmit(a.ctx, "online_list", users)
			// Also notify frontend to refresh contacts
			wailsruntime.EventsEmit(a.ctx, "contacts_changed")
		}
	}
}

// isUserOnline checks both LAN (direct TCP) and relay presence.
func (a *App) isUserOnline(uuid string) bool {
	// Check LAN connection
	if a.client.IsPeerOnline(uuid) {
		return true
	}
	// Check relay-reported online status
	a.onlineMu.RLock()
	defer a.onlineMu.RUnlock()
	return a.onlineUsers[uuid]
}

// shutdown is called by Wails when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.client != nil {
		a.client.Stop()
	}
}

// --- Wails Bindings: callable from React frontend via wailsjs ---

// ContactInfo is the JSON-friendly struct sent to the frontend.
type ContactInfo struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

// MessageInfo is the JSON-friendly struct for chat messages.
type MessageInfo struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Encrypted bool   `json:"encrypted"`
	IsMe      bool   `json:"isMe"`
}

// ReactionInfo is the JSON-friendly struct for message reactions.
type ReactionInfo struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// GetLocalUser returns the current user's info.
func (a *App) GetLocalUser() (*ContactInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	profile := a.client.GetProfile()
	if profile == nil {
		return nil, fmt.Errorf("no profile registered")
	}
	return &ContactInfo{
		UUID:     profile.UUID,
		Username: profile.Username,
	}, nil
}

// Register creates a new user profile.
func (a *App) Register(username string) (*ContactInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	profile, err := a.client.Register(username)
	if err != nil {
		return nil, err
	}
	// Start networking after registration
	if err := a.client.Start(a.ctx); err != nil {
		log.Printf("desktop: failed to start after register: %v", err)
	}
	go a.eventLoop(profile)
	return &ContactInfo{
		UUID:     profile.UUID,
		Username: profile.Username,
	}, nil
}

// GetContacts returns all contacts with online status.
func (a *App) GetContacts() ([]ContactInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	contacts, err := a.client.ListContacts()
	if err != nil {
		return nil, err
	}
	result := make([]ContactInfo, len(contacts))
	for i, c := range contacts {
		result[i] = ContactInfo{
			UUID:     c.UUID,
			Username: c.Username,
			Online:   a.isUserOnline(c.UUID),
		}
	}
	return result, nil
}

// GetChatHistory returns messages for a conversation.
func (a *App) GetChatHistory(contactUUID string) ([]MessageInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	profile := a.client.GetProfile()
	if profile == nil {
		return nil, fmt.Errorf("no profile")
	}
	history, err := a.client.GetChatHistory(contactUUID)
	if err != nil {
		return nil, err
	}
	result := make([]MessageInfo, len(history))
	for i, msg := range history {
		result[i] = MessageInfo{
			ID:        msg.ID,
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp.Format(time.RFC3339),
			Status:    msg.Status,
			Encrypted: msg.Encrypted,
			IsMe:      msg.Sender == profile.UUID,
		}
	}
	return result, nil
}

// SendMessage sends a text message to a contact.
func (a *App) SendMessage(contactUUID string, content string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	content = ui.ReplaceShortcodes(content)
	return a.client.SendMessage(contactUUID, content)
}

// AddContact adds a new contact by username and UUID.
func (a *App) AddContact(username, uuid string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.AddContact(username, uuid)
}

// DeleteContact removes a contact.
func (a *App) DeleteContact(uuid string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.DeleteContact(uuid)
}

// SendReaction sends an emoji reaction to a message.
func (a *App) SendReaction(contactUUID, messageID, emoji string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.SendReaction(contactUUID, messageID, emoji)
}

// GetChatReactions returns reactions grouped by message ID.
func (a *App) GetChatReactions(contactUUID string) (map[string][]ReactionInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	rawReactions, err := a.client.GetChatReactions(contactUUID)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]ReactionInfo)
	for msgID, reactions := range rawReactions {
		counts := make(map[string]int)
		var order []string
		for _, r := range reactions {
			if counts[r.Emoji] == 0 {
				order = append(order, r.Emoji)
			}
			counts[r.Emoji]++
		}
		for _, emoji := range order {
			result[msgID] = append(result[msgID], ReactionInfo{
				Emoji: emoji,
				Count: counts[emoji],
			})
		}
	}
	return result, nil
}

// GetUnreadCount returns the unread message count for a contact.
func (a *App) GetUnreadCount(contactUUID string) (int, error) {
	if a.client == nil {
		return 0, fmt.Errorf("client not initialized")
	}
	return a.client.GetUnreadCount(contactUUID)
}

// SendTyping sends a typing indicator to a contact.
func (a *App) SendTyping(contactUUID string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.SendTyping(contactUUID)
}

// SearchUsers searches for users on the relay.
func (a *App) SearchUsers(query string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.SearchUsers(query)
}

// GetOnlineUsers requests the online users list from the relay.
func (a *App) GetOnlineUsers() error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return a.client.GetOnlineUsers()
}

// MarkMessagesRead marks messages as read locally and sends read receipts.
func (a *App) MarkMessagesRead(contactUUID string) error {
	if a.client == nil {
		return fmt.Errorf("client not initialized")
	}
	profile := a.client.GetProfile()
	if profile == nil {
		return fmt.Errorf("no profile")
	}
	// Get unread messages from this contact
	history, err := a.client.GetChatHistory(contactUUID)
	if err != nil {
		return err
	}
	var unreadIDs []string
	for _, msg := range history {
		if msg.Sender == contactUUID && msg.Status != "read" {
			unreadIDs = append(unreadIDs, msg.ID)
		}
	}
	if len(unreadIDs) == 0 {
		return nil
	}
	// Mark as read in DB
	if err := a.client.MarkMessagesRead(unreadIDs); err != nil {
		return err
	}
	// Send read receipts to the sender
	_ = a.client.SendReadAck(contactUUID, unreadIDs)
	return nil
}
