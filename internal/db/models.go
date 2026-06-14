package db

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NewProfile creates a Profile with a freshly generated UUID.
func NewProfile(username string) (*Profile, error) {
	return &Profile{
		UUID:     uuid.New().String(),
		Username: username,
	}, nil
}

// Profile represents the local user's profile.
type Profile struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

// Contact represents a known peer (friend).
type Contact struct {
	UUID     string    `json:"uuid"`
	Username string    `json:"username"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

// Message represents a direct message between the local user and a contact.
type Message struct {
	ID        string    `json:"id"` // Unique SHA-256 hash of Content + Sender + Recipient + Timestamp
	Sender    string    `json:"sender_uuid"`
	Recipient string    `json:"recipient_uuid"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "draft", "queued", "synced"
}

// GenerateID computes the unique SHA-256 hash for a message to prevent duplicates and ensure integrity.
func (m *Message) GenerateID() string {
	raw := fmt.Sprintf("%s|%s|%s|%d", m.Sender, m.Recipient, m.Content, m.Timestamp.UnixNano())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
