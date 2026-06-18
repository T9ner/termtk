package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termtalk/internal/crypto"

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
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
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

	// Add X25519 public key columns (US-3 E2E encryption) — backward compatible.
	if err := d.addColumnIfNotExists("profile", "x25519_public_key", "BLOB"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("contacts", "x25519_public_key", "BLOB"); err != nil {
		return err
	}
	// Add encrypted flag to messages table.
	if err := d.addColumnIfNotExists("messages", "encrypted", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	// Reactions table (v0.5.0)
	_, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS reactions (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL,
		sender_uuid TEXT NOT NULL,
		emoji TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		UNIQUE(message_id, sender_uuid, emoji)
	)`)
	if err != nil {
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
	var pubKey, privKey, x25519PubKey []byte
	err := d.conn.QueryRow("SELECT uuid, username, public_key, private_key, x25519_public_key FROM profile LIMIT 1").
		Scan(&p.UUID, &p.Username, &pubKey, &privKey, &x25519PubKey)
	if err == sql.ErrNoRows {
		return nil, nil // No profile registered yet
	}
	if err != nil {
		return nil, err
	}
	p.PublicKey = []byte(pubKey)
	p.PrivateKey = []byte(privKey)
	p.X25519PublicKey = []byte(x25519PubKey)

	// Auto-derive X25519 public key for existing profiles upgrading from pre-US-3
	if len(p.X25519PublicKey) == 0 && len(p.PrivateKey) > 0 {
		x25519Pub, err := crypto.Ed25519ToX25519Public(p.PrivateKey)
		if err == nil {
			p.X25519PublicKey = x25519Pub[:]
			_ = d.SaveProfile(&p)
		}
	}

	return &p, nil
}

// SaveProfile creates or updates the local user's profile.
func (d *Database) SaveProfile(p *Profile) error {
	_, err := d.conn.Exec(
		`INSERT INTO profile (uuid, username, public_key, private_key, x25519_public_key) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(uuid) DO UPDATE SET username = excluded.username, public_key = excluded.public_key, private_key = excluded.private_key, x25519_public_key = excluded.x25519_public_key`,
		p.UUID, p.Username, p.PublicKey, p.PrivateKey, p.X25519PublicKey,
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
		`INSERT INTO contacts (uuid, username, ip, port, last_seen, public_key, verified, x25519_public_key) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) 
		 ON CONFLICT(uuid) DO UPDATE SET 
			username = excluded.username, 
			ip = excluded.ip, 
			port = excluded.port, 
			last_seen = excluded.last_seen,
			public_key = COALESCE(excluded.public_key, contacts.public_key),
			verified = excluded.verified,
			x25519_public_key = COALESCE(excluded.x25519_public_key, contacts.x25519_public_key)`,
		c.UUID, c.Username, c.IP, c.Port, c.LastSeen, c.PublicKey, verified, c.X25519PublicKey,
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
	var pubKey, x25519PubKey []byte
	var verified int
	err := d.conn.QueryRow("SELECT uuid, username, ip, port, last_seen, public_key, verified, x25519_public_key FROM contacts WHERE uuid = ?", uuid).
		Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen, &pubKey, &verified, &x25519PubKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.PublicKey = []byte(pubKey)
	c.Verified = verified != 0
	c.X25519PublicKey = []byte(x25519PubKey)
	return &c, nil
}

// ListContacts retrieves all stored contacts.
func (d *Database) ListContacts() ([]Contact, error) {
	rows, err := d.conn.Query("SELECT uuid, username, ip, port, last_seen, public_key, verified, x25519_public_key FROM contacts ORDER BY username ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var pubKey, x25519PubKey []byte
		var verified int
		if err := rows.Scan(&c.UUID, &c.Username, &c.IP, &c.Port, &c.LastSeen, &pubKey, &verified, &x25519PubKey); err != nil {
			return nil, err
		}
		c.PublicKey = []byte(pubKey)
		c.Verified = verified != 0
		c.X25519PublicKey = []byte(x25519PubKey)
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
	var encrypted int
	if m.Encrypted {
		encrypted = 1
	}
	_, err := tx.Exec(
		`INSERT INTO messages (id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted) 
		 VALUES (?, ?, ?, ?, ?, ?, ?) 
		 ON CONFLICT(id) DO UPDATE SET status = excluded.status`,
		m.ID, m.Sender, m.Recipient, m.Content, m.Timestamp, m.Status, encrypted,
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
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted 
		 FROM (
			 SELECT id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted 
			 FROM messages 
			 WHERE sender_uuid = ? AND recipient_uuid = ?
			 UNION ALL
			 SELECT id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted 
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
		var encrypted int
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status, &encrypted); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		m.Encrypted = encrypted != 0
		history = append(history, m)
	}
	return history, nil
}

// GetUnsyncedMessages retrieves all messages to/from a peer that are not yet marked as 'synced'.
func (d *Database) GetUnsyncedMessages(contactUUID string) ([]Message, error) {
	rows, err := d.conn.Query(
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted 
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
		var encrypted int
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status, &encrypted); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		m.Encrypted = encrypted != 0
		unsynced = append(unsynced, m)
	}
	return unsynced, nil
}

// GetQueuedMessages retrieves all outgoing messages from the local user that
// are still in 'queued' status, for relay outbox drain on reconnect.
func (d *Database) GetQueuedMessages(senderUUID string) ([]Message, error) {
	rows, err := d.conn.Query(
		`SELECT id, sender_uuid, recipient_uuid, content, timestamp, status, encrypted 
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
		var encrypted int
		if err := rows.Scan(&m.ID, &m.Sender, &m.Recipient, &m.Content, &ts, &m.Status, &encrypted); err != nil {
			return nil, err
		}
		m.Timestamp = ts
		m.Encrypted = encrypted != 0
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

// SetContactVerified sets the verified flag for a contact.
func (d *Database) SetContactVerified(uuid string, verified bool) error {
	v := 0
	if verified {
		v = 1
	}
	_, err := d.conn.Exec("UPDATE contacts SET verified = ? WHERE uuid = ?", v, uuid)
	return err
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

// DeleteMessages removes messages by their IDs from the local database.
func (d *Database) DeleteMessages(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM messages WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := d.conn.Exec(query, args...)
	return err
}

// AddReaction inserts a reaction into the database.
func (d *Database) AddReaction(id, messageID, senderUUID, emoji, timestamp string) error {
	_, err := d.conn.Exec(
		`INSERT INTO reactions (id, message_id, sender_uuid, emoji, timestamp)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(message_id, sender_uuid, emoji) DO NOTHING`,
		id, messageID, senderUUID, emoji, timestamp,
	)
	return err
}

// RemoveReaction deletes a reaction from the database.
func (d *Database) RemoveReaction(messageID, senderUUID, emoji string) error {
	_, err := d.conn.Exec(
		"DELETE FROM reactions WHERE message_id = ? AND sender_uuid = ? AND emoji = ?",
		messageID, senderUUID, emoji,
	)
	return err
}

// GetReactions returns all reactions for a given message.
func (d *Database) GetReactions(messageID string) ([]Reaction, error) {
	rows, err := d.conn.Query(
		"SELECT id, message_id, sender_uuid, emoji, timestamp FROM reactions WHERE message_id = ? ORDER BY timestamp ASC",
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.ID, &r.MessageID, &r.SenderUUID, &r.Emoji, &r.Timestamp); err != nil {
			return nil, err
		}
		reactions = append(reactions, r)
	}
	return reactions, nil
}

// GetChatReactions returns a map of messageID to reactions for all messages in a chat.
func (d *Database) GetChatReactions(senderUUID, recipientUUID string) (map[string][]Reaction, error) {
	rows, err := d.conn.Query(
		`SELECT r.id, r.message_id, r.sender_uuid, r.emoji, r.timestamp
		 FROM reactions r
		 INNER JOIN messages m ON r.message_id = m.id
		 WHERE (m.sender_uuid = ? AND m.recipient_uuid = ?)
		    OR (m.sender_uuid = ? AND m.recipient_uuid = ?)
		 ORDER BY r.timestamp ASC`,
		senderUUID, recipientUUID, recipientUUID, senderUUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Reaction)
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.ID, &r.MessageID, &r.SenderUUID, &r.Emoji, &r.Timestamp); err != nil {
			return nil, err
		}
		result[r.MessageID] = append(result[r.MessageID], r)
	}
	return result, nil
}

// GetSetting retrieves a setting value by key. Returns empty string if not found.
func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting upserts a setting key-value pair.
func (d *Database) SetSetting(key, value string) error {
	_, err := d.conn.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
