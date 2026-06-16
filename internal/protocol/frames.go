package protocol

import "encoding/json"

// RelayFrame represents the message wrapper used by the relay server.
// Both the relay server (cmd/termtalk-relay) and the client sync manager
// (internal/network/sync.go) use this shared type to encode/decode
// relay protocol messages.
type RelayFrame struct {
	Type      string          `json:"type"`                // "register", "relay", "msg", "offline", "ping"
	UUID      string          `json:"uuid,omitempty"`      // Client registration UUID
	Username  string          `json:"username,omitempty"`  // Client registration Username
	Recipient string          `json:"recipient,omitempty"` // Target Recipient UUID
	Message   json.RawMessage `json:"message,omitempty"`   // Nested Frame payload
}
