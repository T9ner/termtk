package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	pragmas := `
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA temp_store=MEMORY;
		PRAGMA cache_size=-64000;
		PRAGMA busy_timeout=5000;`
	_, err = db.Exec(pragmas)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure sqlite pragma: %w", err)
	}

	// Prevent SQLITE_BUSY deadlocks by serializing connection access in Go
	db.SetMaxOpenConns(1)

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
		`CREATE INDEX IF NOT EXISTS idx_messages_unsynced_sender ON messages(sender_uuid) WHERE status != 'synced';`,
		`CREATE INDEX IF NOT EXISTS idx_messages_unsynced_recipient ON messages(recipient_uuid) WHERE status != 'synced';`,
	}

	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return err
		}
	}

	// Add Ed25519 identity columns (v0.4.0) — backward compatible with existing DBs.
	if err := d.addColumnIfNotExists("profile", "public_key", "BLOB"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("profile", "private_key", "BLOB"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("contacts", "public_key", "BLOB"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("contacts", "verified", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	return nil
}

// addColumnIfNotExists adds a column to a table, ignoring errors if the column already exists.
func (d *Database) addColumnIfNotExists(table, column, colType string) error {
	_, err := d.conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	if err != nil && strings.Contains(err.Error(), "duplicate column") {
		return nil // Column already exists, safe to ignore
	}
	return err
}

// GetProfile retrieves the local user's profile if it exists.
func (d *Database) GetProfile() (*Profile, error) {
	var p Profile
	var pubKey, privKey []byte
	err := d.conn.QueryRow("SELECT uuid, username, public_key, private_key FROM profile LIMIT 1").
		Scan(&p.UUID, &p.Username, &pubKey, &privKey)
	if err == sql.ErrNoRows {
		return nil, nil // No profile registered yet
	}
	if err != nil {
		return nil, err
	}
	p.PublicKey = []byte(pubKey)
	p.PrivateKey = []byte(privKey)
	return &p, nil
}

// SaveProfile creates or updates the local user's profile.
func (d *Database) SaveProfile(p *Profile) error {
	_, err := d.conn.Exec(
		`INSERT INTO profile (uuid, username, public_key, private_key) VALUES (?, ?, ?, ?)
		 ON CONFLICT(uuid) DO UPDATE SET username = excluded.username, public_key = excluded.public_key, private_key = excluded.private_key`,
		p.UUID, p.Username, p.PublicKey, p.PrivateKey,
	)
	return err
}

// upsertContactTx inserts or updates a peer's details in the contacts table inside a transaction.
func (d *Database) upsertContactTx(tx *sql.Tx, c *Contact) error {
	var verified int
	if c.Verified {
		verified = 1
	}
	_, err := tx.Exec(
		`INSERT INTO contacts (uuid, username, ip, port, last_seen, public_key, verified) 
		 VALUES (?, ?, ?, ?, ?, ?, ?) 
		 ON CONFLICT(uuid) DO UPDATE SET 
			username = excluded.username, 
			ip = excluded.ip, 
			port = excluded.port, 
			last_seen = excluded.last_seen,
			public_key = COALESCE(excluded.public_key, contacts.public_key),
			verified = excluded.verified`,
		c.UUID, c.Username, c.IP, c.Port, c.LastSeen, c.PublicKey, verified,
	)
	return err
}

// UpsertContact inserts or updates a peer's details in the contacts table.
func (d *Database) UpsertContact(c *Contact) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := d.upsertContactTx(tx, c); err != nil {
		return err
	}
	return tx.Commit()
}

// GetContact retrieves a single contact by their UUID.
func (d *Database) GetContact(uuid string) (*Contact, error) {
	var c Contact
	var pubKey []byte
	var verified int
	err := d.conn.QueryRow("SELECT uuid, username, ip, port, last_seen, public_key, verified FROM contacts WHERE uuid = ?", uuid).
		Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen, &pubKey, &verified)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.PublicKey = []byte(pubKey)
	c.Verified = verified != 0
	return &c, nil
}

// ListContacts retrieves all stored contacts.
func (d *Database) ListContacts() ([]Contact, error) {
	rows, err := d.conn.Query("SELECT uuid, username, ip, port, last_seen, public_key, verified FROM contacts ORDER BY username ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var pubKey []byte
		var verified int
		if err := rows.Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen, &pubKey, &verified); err != nil {
			return nil, err
		}
		c.PublicKey = []byte(pubKey)
		c.Verified = verified != 0
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// DeleteContact removes a contact by UUID. Message history is preserved.
func (d *Database) DeleteContact(uuid string) error {
	_, err := d.conn.Exec("DELETE FROM contacts WHERE uuid = ?", uuid)
	return err
}

// saveMessageTx stores a message in the database inside a transaction.
func (d *Database) saveMessageTx(tx *sql.Tx, m *Message) error {
	if m.ID == "" {
		m.ID = m.GenerateID()
	}
	_, err := tx.Exec(
		`INSERT INTO messages (id, sender_uuid, recipient_uuid, content, timestamp, status) 
		 VALUES (?, ?, ?, ?, ?, ?) 
		 ON CONFLICT(id) DO UPDATE SET status = excluded.status`,
		m.ID, m.Sender, m.Recipient, m.Content, m.Timestamp, m.Status,
	)
	return err
}

// SaveMessage stores a message in the database.
func (d *Database) SaveMessage(m *Message) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := d.saveMessageTx(tx, m); err != nil {
		return err
	}
	return tx.Commit()
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

// GetQueuedMessages retrieves all outgoing messages from the local user that
// are still in 'queued' status, for relay outbox drain on reconnect.
func (d *Database) GetQueuedMessages(senderUUID string) ([]Message, error) {
	rows, err := d.conn.Query(
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
		 FROM messages 
		 WHERE sender_uuid = ? AND status = 'queued'
		 ORDER BY timestamp ASC`,
		senderUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queued []Message
	for rows.Next() {
		var m Message
		var ts time.Time
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		queued = append(queued, m)
	}
	return queued, nil
}

// GetUnreadCount returns the number of messages from contactUUID to localUUID
// that have not been marked as 'read'.
func (d *Database) GetUnreadCount(localUUID, contactUUID string) (int, error) {
	var count int
	err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE sender_uuid = ? AND recipient_uuid = ? AND status != 'read'`,
		contactUUID, localUUID,
	).Scan(&count)
	return count, err
}

// MarkMessagesRead updates the status of specific messages to 'read' in a transaction.
func (d *Database) MarkMessagesRead(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range messageIDs {
		if _, err := tx.Exec("UPDATE messages SET status = 'read' WHERE id = ?", id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
