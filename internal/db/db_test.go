package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) (*Database, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nod_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	database, err := NewDatabase(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create test database: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.RemoveAll(dir)
	}

	return database, cleanup
}

func TestProfile(t *testing.T) {
	database, cleanup := tempDB(t)
	defer cleanup()

	// 1. Get profile when none exists
	prof, err := database.GetProfile()
	if err != nil {
		t.Errorf("GetProfile err: %v", err)
	}
	if prof != nil {
		t.Errorf("expected nil profile, got %+v", prof)
	}

	// 2. Save profile
	p := &Profile{
		UUID:     "user-123",
		Username: "alice",
	}
	err = database.SaveProfile(p)
	if err != nil {
		t.Fatalf("SaveProfile err: %v", err)
	}

	// 3. Load profile and verify
	prof, err = database.GetProfile()
	if err != nil {
		t.Errorf("GetProfile err: %v", err)
	}
	if prof == nil || prof.UUID != p.UUID || prof.Username != p.Username {
		t.Errorf("loaded profile mismatch: expected %+v, got %+v", p, prof)
	}

	// 4. Update profile (conflict)
	p.Username = "alice_new"
	err = database.SaveProfile(p)
	if err != nil {
		t.Fatalf("SaveProfile update err: %v", err)
	}

	prof, err = database.GetProfile()
	if err != nil {
		t.Errorf("GetProfile err: %v", err)
	}
	if prof.Username != "alice_new" {
		t.Errorf("expected updated username alice_new, got %s", prof.Username)
	}
}

func TestContacts(t *testing.T) {
	database, cleanup := tempDB(t)
	defer cleanup()

	c := &Contact{
		UUID:     "peer-abc",
		Username: "bob",
		IP:       "192.168.1.50",
		Port:     55556,
		LastSeen: time.Now().Round(time.Second), // Round for simple DB comparison
	}

	// 1. Save contact
	err := database.UpsertContact(c)
	if err != nil {
		t.Fatalf("UpsertContact err: %v", err)
	}

	// 2. Retrieve contact
	loaded, err := database.GetContact(c.UUID)
	if err != nil {
		t.Errorf("GetContact err: %v", err)
	}
	if loaded == nil || loaded.UUID != c.UUID || loaded.Username != c.Username || loaded.IP != c.IP || loaded.Port != c.Port {
		t.Errorf("contact mismatch: expected %+v, got %+v", c, loaded)
	}

	// 3. List contacts
	list, err := database.ListContacts()
	if err != nil {
		t.Errorf("ListContacts err: %v", err)
	}
	if len(list) != 1 || list[0].UUID != c.UUID {
		t.Errorf("expected 1 contact, got %d", len(list))
	}
}

func TestMessages(t *testing.T) {
	database, cleanup := tempDB(t)
	defer cleanup()

	localUUID := "me-123"
	friendUUID := "friend-456"

	msg := &Message{
		Sender:    localUUID,
		Recipient: friendUUID,
		Content:   "Hello friend!",
		Timestamp: time.Now().Round(time.Second),
		Status:    "queued",
	}

	// 1. Generate ID and save message
	msg.ID = msg.GenerateID()
	err := database.SaveMessage(msg)
	if err != nil {
		t.Fatalf("SaveMessage err: %v", err)
	}

	// 2. Get history and check
	history, err := database.GetChatHistory(localUUID, friendUUID, 0, 0)
	if err != nil {
		t.Fatalf("GetChatHistory err: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message in history, got %d", len(history))
	}
	if history[0].ID != msg.ID || history[0].Content != msg.Content || history[0].Status != msg.Status {
		t.Errorf("message content mismatch: expected %+v, got %+v", msg, history[0])
	}

	// 3. Update status
	err = database.UpdateMessageStatus(msg.ID, "synced")
	if err != nil {
		t.Fatalf("UpdateMessageStatus err: %v", err)
	}

	// 4. Verify status updated
	history, err = database.GetChatHistory(localUUID, friendUUID, 0, 0)
	if err != nil {
		t.Fatalf("GetChatHistory err: %v", err)
	}
	if history[0].Status != "synced" {
		t.Errorf("expected status synced, got %s", history[0].Status)
	}

	// 5. Test duplicates insertion (no-op or update status)
	dup := *msg
	dup.Status = "draft"
	err = database.SaveMessage(&dup)
	if err != nil {
		t.Fatalf("SaveMessage duplicate err: %v", err)
	}

	history, err = database.GetChatHistory(localUUID, friendUUID, 0, 0)
	if err != nil {
		t.Fatalf("GetChatHistory err: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 message (deduplicated), got %d", len(history))
	}
	// It should update the status to "draft"
	if history[0].Status != "draft" {
		t.Errorf("expected status to update to draft, got %s", history[0].Status)
	}
}
