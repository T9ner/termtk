package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"termtalk/internal/protocol"
)

// RelayStore provides SQLite persistence for the relay server.
type RelayStore struct {
	db *sql.DB
}

func NewRelayStore(dbPath string) (*RelayStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("relay store: %w", err)
	}
	// Same pragmas as client DB
	db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;")
	db.SetMaxOpenConns(1)

	s := &RelayStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *RelayStore) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			uuid TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			public_key TEXT,
			last_seen DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS stored_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sender_uuid TEXT NOT NULL,
			sender_username TEXT NOT NULL,
			recipient_uuid TEXT NOT NULL,
			payload TEXT NOT NULL,
			message_id TEXT,
			created_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_stored_recipient ON stored_messages(recipient_uuid);`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *RelayStore) Close() error { return s.db.Close() }

// UpsertUser saves or updates a user registration.
func (s *RelayStore) UpsertUser(uuid, username, publicKey string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (uuid, username, public_key, last_seen) VALUES (?, ?, ?, ?)
		 ON CONFLICT(uuid) DO UPDATE SET username=excluded.username, public_key=excluded.public_key, last_seen=excluded.last_seen`,
		uuid, username, publicKey, time.Now(),
	)
	return err
}

// LoadUsers returns all registered users.
func (s *RelayStore) LoadUsers() (map[string]RegisteredUser, error) {
	rows, err := s.db.Query("SELECT uuid, username, public_key, last_seen FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make(map[string]RegisteredUser)
	for rows.Next() {
		var u RegisteredUser
		var pubKey sql.NullString
		if err := rows.Scan(&u.UUID, &u.Username, &pubKey, &u.LastSeen); err != nil {
			return nil, err
		}
		if pubKey.Valid {
			u.PublicKey = pubKey.String
		}
		users[u.UUID] = u
	}
	return users, nil
}

// StoreMessage persists a message for an offline recipient.
func (s *RelayStore) StoreMessage(senderUUID, senderUsername, recipientUUID string, frame protocol.RelayFrame) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO stored_messages (sender_uuid, sender_username, recipient_uuid, payload, message_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		senderUUID, senderUsername, recipientUUID, string(payload), frame.MessageID, time.Now(),
	)
	return err
}

// LoadAndDeleteMessages loads all stored messages for a recipient and deletes them.
func (s *RelayStore) LoadAndDeleteMessages(recipientUUID string) ([]StoredMessage, error) {
	rows, err := s.db.Query(
		`SELECT sender_uuid, sender_username, payload, created_at FROM stored_messages WHERE recipient_uuid = ? ORDER BY created_at ASC`,
		recipientUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []StoredMessage
	for rows.Next() {
		var sm StoredMessage
		var payload string
		if err := rows.Scan(&sm.SenderUUID, &sm.SenderUsername, &payload, &sm.StoredAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(payload), &sm.Frame); err != nil {
			continue // skip corrupted
		}
		messages = append(messages, sm)
	}
	// Delete after successful read
	_, _ = s.db.Exec("DELETE FROM stored_messages WHERE recipient_uuid = ?", recipientUUID)
	return messages, nil
}

// StoredMessageCount returns total stored messages (for health check).
func (s *RelayStore) StoredMessageCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM stored_messages").Scan(&count)
	return count, err
}
