package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tempSyncPath returns a temp file path for a sync file and a cleanup func.
func tempSyncPath(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nod_sneakernet_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return filepath.Join(dir, "sync.json"), func() { os.RemoveAll(dir) }
}

// seedMessage inserts a message with the given status into the database.
func seedMessage(t *testing.T, database *Database, sender, recipient, content, status string) *Message {
	t.Helper()
	msg := &Message{
		Sender:    sender,
		Recipient: recipient,
		Content:   content,
		Timestamp: time.Now().Round(time.Second),
		Status:    status,
	}
	msg.ID = msg.GenerateID()
	if err := database.SaveMessage(msg); err != nil {
		t.Fatalf("seedMessage: SaveMessage err: %v", err)
	}
	return msg
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 1 (tracer bullet): ExportSyncFile writes a file with the unsynced messages.
// ──────────────────────────────────────────────────────────────────────────────

func TestExportSyncFile_WritesFile(t *testing.T) {
	database, cleanup := tempDB(t)
	defer cleanup()

	aliceProfile := &Profile{UUID: "alice-uuid", Username: "alice"}
	bobUUID := "bob-uuid"

	// Seed one queued message from alice to bob
	seedMessage(t, database, aliceProfile.UUID, bobUUID, "Hey Bob!", "queued")

	syncPath, syncCleanup := tempSyncPath(t)
	defer syncCleanup()

	err := database.ExportSyncFile(bobUUID, aliceProfile, syncPath)
	if err != nil {
		t.Fatalf("ExportSyncFile err: %v", err)
	}

	// File must exist and be non-empty
	info, err := os.Stat(syncPath)
	if err != nil {
		t.Fatalf("expected sync file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected sync file to be non-empty")
	}

	// File must be valid JSON with at least one message
	data, _ := os.ReadFile(syncPath)
	var sf SyncFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("sync file is not valid JSON: %v", err)
	}
	if len(sf.Messages) != 1 {
		t.Errorf("expected 1 message in sync file, got %d", len(sf.Messages))
	}
	if sf.Messages[0].Content != "Hey Bob!" {
		t.Errorf("unexpected message content: %s", sf.Messages[0].Content)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 2: Full round-trip — exported messages appear in the importer's database.
// ──────────────────────────────────────────────────────────────────────────────

func TestSyncFile_RoundTrip(t *testing.T) {
	aliceDB, aliceCleanup := tempDB(t)
	defer aliceCleanup()

	bobDB, bobCleanup := tempDB(t)
	defer bobCleanup()

	aliceProfile := &Profile{UUID: "alice-uuid", Username: "alice"}
	bobUUID := "bob-uuid"

	// Alice seeds two queued messages destined for Bob
	seedMessage(t, aliceDB, aliceProfile.UUID, bobUUID, "Hello Bob!", "queued")
	seedMessage(t, aliceDB, aliceProfile.UUID, bobUUID, "Are you there?", "queued")

	syncPath, syncCleanup := tempSyncPath(t)
	defer syncCleanup()

	// Alice exports
	if err := aliceDB.ExportSyncFile(bobUUID, aliceProfile, syncPath); err != nil {
		t.Fatalf("ExportSyncFile err: %v", err)
	}

	// Bob imports
	file, err := bobDB.ImportSyncFile(syncPath)
	if err != nil {
		t.Fatalf("ImportSyncFile err: %v", err)
	}

	// Imported SyncFile reports the right sender
	if file.SenderUUID != aliceProfile.UUID {
		t.Errorf("expected sender uuid %s, got %s", aliceProfile.UUID, file.SenderUUID)
	}
	if file.SenderUsername != aliceProfile.Username {
		t.Errorf("expected sender username %s, got %s", aliceProfile.Username, file.SenderUsername)
	}

	// Bob's database now contains both messages
	history, err := bobDB.GetChatHistory(bobUUID, aliceProfile.UUID, 0, 0)
	if err != nil {
		t.Fatalf("GetChatHistory err: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages in bob's history, got %d", len(history))
	}

	contents := map[string]bool{}
	for _, m := range history {
		contents[m.Content] = true
	}
	if !contents["Hello Bob!"] || !contents["Are you there?"] {
		t.Errorf("message contents mismatch: got %v", contents)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 3: Hash integrity — a tampered message is rejected on import.
// ──────────────────────────────────────────────────────────────────────────────

func TestImportSyncFile_RejectsHashMismatch(t *testing.T) {
	aliceDB, aliceCleanup := tempDB(t)
	defer aliceCleanup()

	bobDB, bobCleanup := tempDB(t)
	defer bobCleanup()

	aliceProfile := &Profile{UUID: "alice-uuid", Username: "alice"}
	bobUUID := "bob-uuid"
	seedMessage(t, aliceDB, aliceProfile.UUID, bobUUID, "Tamper me", "queued")

	syncPath, syncCleanup := tempSyncPath(t)
	defer syncCleanup()

	if err := aliceDB.ExportSyncFile(bobUUID, aliceProfile, syncPath); err != nil {
		t.Fatalf("ExportSyncFile err: %v", err)
	}

	// Tamper: read the file, change the message content, write it back
	data, _ := os.ReadFile(syncPath)
	var sf SyncFile
	_ = json.Unmarshal(data, &sf)
	sf.Messages[0].Content = "I have been tampered with"
	tampered, _ := json.Marshal(sf)
	_ = os.WriteFile(syncPath, tampered, 0644)

	// Import should fail due to hash mismatch
	_, err := bobDB.ImportSyncFile(syncPath)
	if err == nil {
		t.Fatal("expected ImportSyncFile to reject tampered message, but it succeeded")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test 4: Sender auto-registers as a Contact in the importer's database.
// ──────────────────────────────────────────────────────────────────────────────

func TestImportSyncFile_RegistersSenderAsContact(t *testing.T) {
	aliceDB, aliceCleanup := tempDB(t)
	defer aliceCleanup()

	bobDB, bobCleanup := tempDB(t)
	defer bobCleanup()

	aliceProfile := &Profile{UUID: "alice-uuid", Username: "alice"}
	bobUUID := "bob-uuid"
	seedMessage(t, aliceDB, aliceProfile.UUID, bobUUID, "Register me", "queued")

	syncPath, syncCleanup := tempSyncPath(t)
	defer syncCleanup()

	if err := aliceDB.ExportSyncFile(bobUUID, aliceProfile, syncPath); err != nil {
		t.Fatalf("ExportSyncFile err: %v", err)
	}
	if _, err := bobDB.ImportSyncFile(syncPath); err != nil {
		t.Fatalf("ImportSyncFile err: %v", err)
	}

	// Alice should now appear in Bob's contact list
	contact, err := bobDB.GetContact(aliceProfile.UUID)
	if err != nil {
		t.Fatalf("GetContact err: %v", err)
	}
	if contact == nil {
		t.Fatal("expected alice to be registered as a contact in bob's database, got nil")
	}
	if contact.Username != aliceProfile.Username {
		t.Errorf("expected contact username %s, got %s", aliceProfile.Username, contact.Username)
	}
}
