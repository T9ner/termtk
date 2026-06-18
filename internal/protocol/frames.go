package protocol

import "encoding/json"

// RelayFrame represents the message wrapper used by the relay server.
// Both the relay server (cmd/termtalk-relay) and the client sync manager
// (internal/network/sync.go) use this shared type to encode/decode
// relay protocol messages.
//
// Frame types:
//   - "register"       — client→relay: register with UUID and username
//   - "registered"     — relay→client: registration acknowledgement
//   - "relay"          — client→relay: route a message to a recipient
//   - "msg"            — relay→client: incoming message from another peer
//   - "offline"        — relay→client: recipient is not connected
//   - "ping"/"pong"    — keepalive heartbeat
//   - "search"         — client→relay: search for users by username query
//   - "search_result"  — relay→client: list of matching users
//   - "who_online"     — client→relay: request list of online users
//   - "online_list"    — relay→client: list of online users
//   - "list_users"     — client→relay: request full user directory
//   - "user_list"      — relay→client: full user directory response
//   - "stored"         — relay→client: message was stored for offline recipient
//   - "delivered"      — relay→client: stored message was delivered to recipient
//   - "flush"          — relay→client: delivering stored messages on reconnect
//   - "read_ack"       — client→relay→client: batch read receipt for messages
//   - "delete"         — client→relay→client: delete messages by ID
//   - "ice_offer"      — client→relay→client: ICE offer for NAT hole punching (ephemeral)
//   - "ice_answer"     — client→relay→client: ICE answer for NAT hole punching (ephemeral)
type RelayFrame struct {
	Type            string          `json:"type"`                        // Frame type identifier
	UUID            string          `json:"uuid,omitempty"`              // Client registration UUID
	Username        string          `json:"username,omitempty"`          // Client registration Username
	Recipient       string          `json:"recipient,omitempty"`         // Target Recipient UUID
	Message         json.RawMessage `json:"message,omitempty"`           // Nested Frame payload
	Query           string          `json:"query,omitempty"`             // Search query string
	Users           []UserInfo      `json:"users,omitempty"`             // Search/online results
	MessageID       string          `json:"message_id,omitempty"`        // For stored/delivered acks
	MessageIDs      []string        `json:"message_ids,omitempty"`       // For read_ack batches
	PublicKey       string          `json:"public_key,omitempty"`        // Base64 Ed25519 public key
	Signature       string          `json:"signature,omitempty"`         // Base64 Ed25519 signature
	Encrypted       bool            `json:"encrypted,omitempty"`         // True if Message payload is NaCl box encrypted
	Nonce           string          `json:"nonce,omitempty"`             // Base64 NaCl box nonce (24 bytes)
	X25519PublicKey string          `json:"x25519_public_key,omitempty"` // Base64 X25519 public key for encryption
}

// UserInfo represents a user in search/online/directory results.
type UserInfo struct {
	UUID            string `json:"uuid"`
	Username        string `json:"username"`
	Online          bool   `json:"online"`
	LastSeen        string `json:"last_seen,omitempty"`         // ISO 8601 timestamp
	PublicKey       string `json:"public_key,omitempty"`        // Base64 Ed25519 public key
	X25519PublicKey string `json:"x25519_public_key,omitempty"` // Base64 X25519 public key
}
