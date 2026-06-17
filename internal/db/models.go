package db

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NewProfile creates a Profile with a freshly generated UUID and Ed25519 keypair.
func NewProfile(username string) (*Profile, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	return &Profile{
		UUID:       uuid.New().String(),
		Username:   username,
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// Profile represents the local user's profile.
type Profile struct {
	UUID       string `json:"uuid"`
	Username   string `json:"username"`
	PublicKey  []byte `json:"public_key"`
	PrivateKey []byte `json:"private_key"`
}

// Fingerprint returns the first 8 hex characters of the SHA-256 hash of the public key.
// Returns empty string if no public key is set.
func (p *Profile) Fingerprint() string {
	if len(p.PublicKey) == 0 {
		return ""
	}
	hash := sha256.Sum256(p.PublicKey)
	return fmt.Sprintf("%x", hash[:4])
}

// Sign signs the given data with the profile's Ed25519 private key.
// Returns nil if no private key is set.
func (p *Profile) Sign(data []byte) []byte {
	if len(p.PrivateKey) == 0 {
		return nil
	}
	return ed25519.Sign(p.PrivateKey, data)
}

// Verify checks an Ed25519 signature against a public key and data.
// Returns false if the public key or signature is invalid or the wrong length.
func Verify(publicKey, data, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, data, signature)
}

// Contact represents a known peer (friend).
type Contact struct {
	UUID      string    `json:"uuid"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	LastSeen  time.Time `json:"last_seen"`
	PublicKey []byte    `json:"public_key"`
	Verified  bool      `json:"verified"`
}

// MessageStatus represents the lifecycle state of a Message.
type MessageStatus string

const (
	StatusDraft  MessageStatus = "draft"
	StatusQueued MessageStatus = "queued"
	StatusStored MessageStatus = "stored" // Relay has it, recipient offline
	StatusSynced MessageStatus = "synced"
	StatusAck    MessageStatus = "ack"
	StatusRead   MessageStatus = "read" // Recipient has opened and read the message
)

// Message represents a direct message between the local user and a contact.
type Message struct {
	ID        string    `json:"id"` // Unique SHA-256 hash of Content + Sender + Recipient + Timestamp
	Sender    string    `json:"sender_uuid"`
	Recipient string    `json:"recipient_uuid"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // MessageStatus: draft, queued, synced, ack
}

// GenerateID computes the unique SHA-256 hash for a message to prevent duplicates and ensure integrity.
func (m *Message) GenerateID() string {
	raw := fmt.Sprintf("%s|%s|%s|%d", m.Sender, m.Recipient, m.Content, m.Timestamp.UnixNano())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
