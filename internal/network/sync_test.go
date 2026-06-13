package network

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"termtalk/internal/db"
)

func createTempDB(t *testing.T, name string) (*db.Database, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "termtalk_net_test_"+name)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	database, err := db.NewDatabase(dbPath)
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

func TestSyncManagerP2P(t *testing.T) {
	// 1. Create DB and SyncManager for Alice
	aliceDB, aliceCleanup := createTempDB(t, "alice")
	defer aliceCleanup()

	aliceProfile := &db.Profile{UUID: "alice-uuid", Username: "alice"}
	err := aliceDB.SaveProfile(aliceProfile)
	if err != nil {
		t.Fatalf("failed to save alice profile: %v", err)
	}

	aliceSync := NewSyncManager(aliceProfile.UUID, aliceProfile.Username, 55561, aliceDB)
	err = aliceSync.Start()
	if err != nil {
		t.Fatalf("failed to start alice sync manager: %v", err)
	}
	defer aliceSync.Stop()

	// 2. Create DB and SyncManager for Bob
	bobDB, bobCleanup := createTempDB(t, "bob")
	defer bobCleanup()

	bobProfile := &db.Profile{UUID: "bob-uuid", Username: "bob"}
	err = bobDB.SaveProfile(bobProfile)
	if err != nil {
		t.Fatalf("failed to save bob profile: %v", err)
	}

	bobSync := NewSyncManager(bobProfile.UUID, bobProfile.Username, 55562, bobDB)
	err = bobSync.Start()
	if err != nil {
		t.Fatalf("failed to start bob sync manager: %v", err)
	}
	defer bobSync.Stop()

	// Register Bob in Alice's contact list so she knows how to dial him
	bobContactForAlice := &db.Contact{
		UUID:     bobProfile.UUID,
		Username: bobProfile.Username,
		IP:       "127.0.0.1",
		Port:     55562,
		LastSeen: time.Now(),
	}
	err = aliceDB.UpsertContact(bobContactForAlice)
	if err != nil {
		t.Fatalf("failed to register bob in alice's contact list: %v", err)
	}

	// Register Alice in Bob's contact list so he knows her
	aliceContactForBob := &db.Contact{
		UUID:     aliceProfile.UUID,
		Username: aliceProfile.Username,
		IP:       "127.0.0.1",
		Port:     55561,
		LastSeen: time.Now(),
	}
	err = bobDB.UpsertContact(aliceContactForBob)
	if err != nil {
		t.Fatalf("failed to register alice in bob's contact list: %v", err)
	}

	// Bind bob message received callback to verify transmission
	messageReceived := make(chan *db.Message, 1)
	bobSync.OnMsgRecv = func(msg *db.Message) {
		messageReceived <- msg
	}

	// 3. Connect Alice to Bob
	err = aliceSync.ConnectToPeer(bobContactForAlice)
	if err != nil {
		t.Fatalf("failed to connect alice to bob: %v", err)
	}

	// Wait for handshake and connection tracking
	time.Sleep(100 * time.Millisecond)

	if !aliceSync.IsPeerOnline(bobProfile.UUID) {
		t.Errorf("alice does not see bob online after connection")
	}

	// 4. Send Message from Alice to Bob
	err = aliceSync.SendMessage(bobProfile.UUID, "Hi Bob, this is Alice!")
	if err != nil {
		t.Fatalf("alice failed to send message: %v", err)
	}

	// Wait for Bob to receive message
	select {
	case msg := <-messageReceived:
		if msg.Content != "Hi Bob, this is Alice!" {
			t.Errorf("expected message content 'Hi Bob, this is Alice!', got '%s'", msg.Content)
		}
		if msg.Sender != aliceProfile.UUID {
			t.Errorf("expected sender '%s', got '%s'", aliceProfile.UUID, msg.Sender)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("bob timed out waiting for message from alice")
	}

	// Verify Bob stored the message in his DB
	history, err := bobDB.GetChatHistory(bobProfile.UUID, aliceProfile.UUID)
	if err != nil {
		t.Fatalf("failed to query bob chat history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message in bob history, got %d", len(history))
	}
	if history[0].Content != "Hi Bob, this is Alice!" {
		t.Errorf("expected message in bob db to be 'Hi Bob, this is Alice!', got '%s'", history[0].Content)
	}
}
