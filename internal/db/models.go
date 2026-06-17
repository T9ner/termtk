package db

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"termtalk/internal/crypto"
)

// VerificationCode computes a 6-digit code from two public keys.
// Both parties compute the same code because keys are sorted before hashing.
func VerificationCode(pubKeyA, pubKeyB []byte) string {
	// Sort keys deterministically
	var first, second []byte
	if bytes.Compare(pubKeyA, pubKeyB) <= 0 {
		first, second = pubKeyA, pubKeyB
	} else {
		first, second = pubKeyB, pubKeyA
	}

	h := sha256.New()
	h.Write(first)
	h.Write(second)
	sum := h.Sum(nil)

	// Take first 4 bytes as uint32, mod 1000000 for 6 digits
	num := uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])
	return fmt.Sprintf("%06d", num%1000000)
}

// NewProfile creates a Profile with a freshly generated UUID and Ed25519 keypair.
func NewProfile(username string) (*Profile, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	x25519Pub, err := crypto.Ed25519ToX25519Public(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to derive X25519 public key: %w", err)
	}
	return &Profile{
		UUID:            uuid.New().String(),
		Username:        username,
		PublicKey:       pub,
		PrivateKey:      priv,
		X25519PublicKey: x25519Pub[:],
	}, nil
}

// Profile represents the local user's profile.
type Profile struct {
	UUID            string `json:"uuid"`
	Username        string `json:"username"`
	PublicKey       []byte `json:"public_key"`
	PrivateKey      []byte `json:"private_key"`
	X25519PublicKey []byte `json:"x25519_public_key"`
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
	UUID            string    `json:"uuid"`
	Username        string    `json:"username"`
	IP              string    `json:"ip"`
	Port            int       `json:"port"`
	LastSeen        time.Time `json:"last_seen"`
	PublicKey       []byte    `json:"public_key"`
	Verified        bool      `json:"verified"`
	X25519PublicKey []byte    `json:"x25519_public_key"`
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
	Status    string    `json:"status"`    // MessageStatus: draft, queued, synced, ack
	Encrypted bool      `json:"encrypted"` // True if this message was sent/received encrypted
}

// GenerateID computes the unique SHA-256 hash for a message to prevent duplicates and ensure integrity.
func (m *Message) GenerateID() string {
	raw := fmt.Sprintf("%s|%s|%s|%d", m.Sender, m.Recipient, m.Content, m.Timestamp.UnixNano())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
