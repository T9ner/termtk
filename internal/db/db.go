package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// Database wraps the SQLite database connection.
type Database struct {
	conn *sql.DB
}

// NewDatabase initializes a new database connection and sets up tables if needed.
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Optimize SQLite for concurrency and durability
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure sqlite pragma: %w", err)
	}

	d := &Database{conn: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return d, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.conn.Close()
}

// migrate creates the necessary tables and indexes.
func (d *Database) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS profile (
			uuid TEXT PRIMARY KEY,
			username TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS contacts (
			uuid TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL,
			last_seen DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			sender_uuid TEXT NOT NULL,
			recipient_uuid TEXT NOT NULL,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			status TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient ON messages(sender_uuid, recipient_uuid, timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_recipient_sender ON messages(recipient_uuid, sender_uuid, timestamp);`,
	}

	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// GetProfile retrieves the local user's profile if it exists.
func (d *Database) GetProfile() (*Profile, error) {
	var p Profile
	err := d.conn.QueryRow("SELECT uuid, username FROM profile LIMIT 1").Scan(&p.UUID, &p.Username)
	if err == sql.ErrNoRows {
		return nil, nil // No profile registered yet
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveProfile creates or updates the local user's profile.
func (d *Database) SaveProfile(p *Profile) error {
	_, err := d.conn.Exec(
		"INSERT INTO profile (uuid, username) VALUES (?, ?) ON CONFLICT(uuid) DO UPDATE SET username = excluded.username",
		p.UUID, p.Username,
	)
	return err
}

// UpsertContact inserts or updates a peer's details in the contacts table.
func (d *Database) UpsertContact(c *Contact) error {
	_, err := d.conn.Exec(
		`INSERT INTO contacts (uuid, username, ip, port, last_seen) 
		 VALUES (?, ?, ?, ?, ?) 
		 ON CONFLICT(uuid) DO UPDATE SET 
			username = excluded.username, 
			ip = excluded.ip, 
			port = excluded.port, 
			last_seen = excluded.last_seen`,
		c.UUID, c.Username, c.IP, c.Port, c.LastSeen,
	)
	return err
}

// GetContact retrieves a single contact by their UUID.
func (d *Database) GetContact(uuid string) (*Contact, error) {
	var c Contact
	err := d.conn.QueryRow("SELECT uuid, username, ip, port, last_seen FROM contacts WHERE uuid = ?", uuid).
		Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListContacts retrieves all stored contacts.
func (d *Database) ListContacts() ([]Contact, error) {
	rows, err := d.conn.Query("SELECT uuid, username, ip, port, last_seen FROM contacts ORDER BY username ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// SaveMessage stores a message in the database.
func (d *Database) SaveMessage(m *Message) error {
	if m.ID == "" {
		m.ID = m.GenerateID()
	}
	_, err := d.conn.Exec(
		`INSERT INTO messages (id, sender_uuid, recipient_uuid, content, timestamp, status) 
		 VALUES (?, ?, ?, ?, ?, ?) 
		 ON CONFLICT(id) DO UPDATE SET status = excluded.status`,
		m.ID, m.Sender, m.Recipient, m.Content, m.Timestamp, m.Status,
	)
	return err
}

// UpdateMessageStatus updates the status of a specific message.
func (d *Database) UpdateMessageStatus(id string, status string) error {
	_, err := d.conn.Exec("UPDATE messages SET status = ? WHERE id = ?", status, id)
	return err
}

// GetChatHistory fetches all messages between the local user and a contact, ordered by time.
func (d *Database) GetChatHistory(localUUID, contactUUID string) ([]Message, error) {
	rows, err := d.conn.Query(
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
		 FROM (
			 SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
			 FROM messages 
			 WHERE sender_uuid = ? AND recipient_uuid = ?
			 UNION ALL
			 SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
			 FROM messages 
			 WHERE sender_uuid = ? AND recipient_uuid = ?
		 )
		 ORDER BY timestamp ASC`,
		localUUID, contactUUID, contactUUID, localUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []Message
	for rows.Next() {
		var m Message
		var ts time.Time
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		history = append(history, m)
	}
	return history, nil
}

// GetUnsyncedMessages retrieves all messages to/from a peer that are not yet marked as 'synced'.
func (d *Database) GetUnsyncedMessages(contactUUID string) ([]Message, error) {
	rows, err := d.conn.Query(
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
		 FROM messages 
		 WHERE (sender_uuid = ? OR recipient_uuid = ?) AND status != 'synced'
		 ORDER BY timestamp ASC`,
		contactUUID, contactUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unsynced []Message
	for rows.Next() {
		var m Message
		var ts time.Time
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		unsynced = append(unsynced, m)
	}
	return unsynced, nil
}
