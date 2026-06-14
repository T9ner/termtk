package client_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"termtalk/internal/client"
)

// newTestClient creates a Client backed by a temp SQLite DB on an ephemeral port.
func newTestClient(t *testing.T, port int) (*client.Client, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "termtalk_client_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "termtalk.db")

	c, err := client.New(dbPath, port)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("client.New: %v", err)
	}

	return c, func() {
		c.Stop()
		os.RemoveAll(dir)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 1 (tracer bullet): Register creates a Profile accessible via GetProfile.
// ──────────────────────────────────────────────────────────────────────────────

func TestClient_Register(t *testing.T) {
	c, cleanup := newTestClient(t, 55570)
	defer cleanup()

	// Before registration, profile is nil
	if c.GetProfile() != nil {
		t.Fatal("expected nil profile before registration")
	}

	prof, err := c.Register("alice")
	if err != nil {
		t.Fatalf("Register err: %v", err)
	}
	if prof.Username != "alice" {
		t.Errorf("expected username alice, got %s", prof.Username)
	}
	if prof.UUID == "" {
		t.Error("expected non-empty UUID after registration")
	}

	// GetProfile should now return the registered profile
	loaded := c.GetProfile()
	if loaded == nil {
		t.Fatal("expected non-nil profile after registration")
	}
	if loaded.UUID != prof.UUID {
		t.Errorf("GetProfile UUID mismatch: %s != %s", loaded.UUID, prof.UUID)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 2: SendMessage persists message; GetChatHistory returns it.
// ──────────────────────────────────────────────────────────────────────────────

func TestClient_SendMessage_PersistedInHistory(t *testing.T) {
	c, cleanup := newTestClient(t, 55571)
	defer cleanup()

	if _, err := c.Register("alice"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	bobUUID := "bob-uuid-001"

	if err := c.SendMessage(bobUUID, "Hello Bob!"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	history, err := c.GetChatHistory(bobUUID)
	if err != nil {
		t.Fatalf("GetChatHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message in history, got %d", len(history))
	}
	if history[0].Content != "Hello Bob!" {
		t.Errorf("unexpected content: %s", history[0].Content)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 3: AddContact + ListContacts round-trip.
// ──────────────────────────────────────────────────────────────────────────────

func TestClient_AddContact_AppearsInList(t *testing.T) {
	c, cleanup := newTestClient(t, 55572)
	defer cleanup()

	if _, err := c.Register("alice"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := c.AddContact("bob", "bob-uuid-002"); err != nil {
		t.Fatalf("AddContact: %v", err)
	}

	contacts, err := c.ListContacts()
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}
	if contacts[0].UUID != "bob-uuid-002" {
		t.Errorf("unexpected contact UUID: %s", contacts[0].UUID)
	}
	if contacts[0].Username != "bob" {
		t.Errorf("unexpected contact Username: %s", contacts[0].Username)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 4: ImportSync merges messages and registers sender as Contact.
// ──────────────────────────────────────────────────────────────────────────────

func TestClient_ImportSync_MergesAndRegistersContact(t *testing.T) {
	// Alice's client — produces the sync file
	aliceClient, aliceCleanup := newTestClient(t, 55573)
	defer aliceCleanup()

	aliceProf, err := aliceClient.Register("alice")
	if err != nil {
		t.Fatalf("alice Register: %v", err)
	}

	// Bob pre-registers with a known UUID so Alice can address him correctly.
	// We do this by creating Bob's client first and registering him, capturing his UUID.
	bobClient, bobCleanup := newTestClient(t, 55574)
	defer bobCleanup()

	bobProf, err := bobClient.Register("bob")
	if err != nil {
		t.Fatalf("bob Register: %v", err)
	}

	// Alice adds Bob as contact and sends him a message (queued — Bob is offline)
	if err := aliceClient.AddContact("bob", bobProf.UUID); err != nil {
		t.Fatalf("alice AddContact: %v", err)
	}
	if err := aliceClient.SendMessage(bobProf.UUID, "Hey Bob, sneakernet!"); err != nil {
		t.Fatalf("alice SendMessage: %v", err)
	}

	// Alice exports the sync file
	syncDir, err := os.MkdirTemp("", "termtalk_sync")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(syncDir)
	syncPath := filepath.Join(syncDir, "sync.json")

	if err := aliceClient.ExportSync(bobProf.UUID, syncPath); err != nil {
		t.Fatalf("ExportSync: %v", err)
	}

	// Bob imports the sync file
	syncFile, err := bobClient.ImportSync(syncPath)
	if err != nil {
		t.Fatalf("ImportSync: %v", err)
	}

	// Imported SyncFile reports correct sender
	if syncFile.SenderUUID != aliceProf.UUID {
		t.Errorf("expected sender uuid %s, got %s", aliceProf.UUID, syncFile.SenderUUID)
	}

	// Alice is now a contact in Bob's database
	contacts, err := bobClient.ListContacts()
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	found := false
	for _, co := range contacts {
		if co.UUID == aliceProf.UUID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected alice (uuid %s) to be registered in bob's contacts after import", aliceProf.UUID)
	}

	// Bob can read Alice's imported message in their shared history
	history, err := bobClient.GetChatHistory(aliceProf.UUID)
	if err != nil {
		t.Fatalf("GetChatHistory: %v", err)
	}
	if len(history) < 1 {
		t.Fatalf("expected at least 1 message in history after import, got %d", len(history))
	}
	if history[0].Content != "Hey Bob, sneakernet!" {
		t.Errorf("unexpected message content: %q", history[0].Content)
	}
	_ = time.Now()
}
