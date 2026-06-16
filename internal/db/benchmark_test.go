package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkGetChatHistory(b *testing.B) {
	// Setup db
	dir, err := os.MkdirTemp("", "termtalk_bench")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "bench.db")
	database, err := NewDatabase(dbPath)
	if err != nil {
		b.Fatalf("failed to create test database: %v", err)
	}
	defer database.Close()

	localUUID := "me"
	targetUUID := "target"

	// Seed the database with 5,000 messages between various users
	// to simulate a realistic usage history.
	for i := 0; i < 5000; i++ {
		sender := fmt.Sprintf("peer-%d", i%20)
		recipient := fmt.Sprintf("peer-%d", (i+1)%20)
		// Inject messages between our target pair
		if i%10 == 0 {
			if i%20 == 0 {
				sender = localUUID
				recipient = targetUUID
			} else {
				sender = targetUUID
				recipient = localUUID
			}
		}
		m := &Message{
			Sender:    sender,
			Recipient: recipient,
			Content:   "some random message content to fill space",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Status:    "synced",
		}
		m.ID = m.GenerateID()
		if err := database.SaveMessage(m); err != nil {
			b.Fatalf("failed to seed db: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history, err := database.GetChatHistory(localUUID, targetUUID)
		if err != nil {
			b.Fatalf("failed to get chat history: %v", err)
		}
		if len(history) == 0 {
			b.Fatalf("expected some history")
		}
	}
}
