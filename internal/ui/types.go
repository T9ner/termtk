package ui

import (
	"termtalk/internal/db"
)

// MsgEvent represents an event received from the background network or database layer.
type MsgEvent interface{}

// PeerDiscoveredMsg is sent when a peer is discovered via UDP.
type PeerDiscoveredMsg struct {
	Contact *db.Contact
}

// MessageReceivedMsg is sent when a message is received via TCP or imported.
type MessageReceivedMsg struct {
	Message *db.Message
}
