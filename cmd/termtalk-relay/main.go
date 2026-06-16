package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"termtalk/internal/protocol"
)

// ClientConn wraps an active TCP connection to a registered client.
type ClientConn struct {
	UUID     string
	Username string
	conn     net.Conn
	enc      *json.Encoder
	mu       sync.Mutex // Protects enc
}

func (cc *ClientConn) Send(frame protocol.RelayFrame) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.enc.Encode(frame)
}

// StoredMessage is a message held for an offline recipient.
type StoredMessage struct {
	SenderUUID     string
	SenderUsername string
	Frame          protocol.RelayFrame
	StoredAt       time.Time
}

// RegisteredUser tracks a user's registration in the directory.
type RegisteredUser struct {
	UUID     string
	Username string
	LastSeen time.Time
}

// RelayServer encapsulates all relay state and logic.
type RelayServer struct {
	clients      map[string]*ClientConn
	clientsMu    sync.RWMutex
	messageStore map[string][]StoredMessage // recipientUUID → stored messages
	storeMu      sync.RWMutex
	userRegistry map[string]RegisteredUser // uuid → user info
	registryMu   sync.RWMutex
}

// NewRelayServer creates a new relay server instance.
func NewRelayServer() *RelayServer {
	return &RelayServer{
		clients:      make(map[string]*ClientConn),
		messageStore: make(map[string][]StoredMessage),
		userRegistry: make(map[string]RegisteredUser),
	}
}

// RegisterClient registers a client, sends ack, and flushes stored messages.
func (rs *RelayServer) RegisterClient(client *ClientConn) {
	rs.clientsMu.Lock()
	if old, exists := rs.clients[client.UUID]; exists {
		old.conn.Close()
	}
	rs.clients[client.UUID] = client
	rs.clientsMu.Unlock()

	// Update user registry
	rs.registryMu.Lock()
	rs.userRegistry[client.UUID] = RegisteredUser{
		UUID:     client.UUID,
		Username: client.Username,
		LastSeen: time.Now(),
	}
	rs.registryMu.Unlock()

	log.Printf("Client registered: %s (%s) from %s", client.Username, client.UUID[:8], client.conn.RemoteAddr())

	// Send registration ack
	if err := client.Send(protocol.RelayFrame{Type: "registered"}); err != nil {
		log.Printf("relay: failed to send registered ack to %s: %v", client.UUID[:8], err)
	}

	// Flush stored messages
	rs.storeMu.Lock()
	stored := rs.messageStore[client.UUID]
	delete(rs.messageStore, client.UUID)
	rs.storeMu.Unlock()

	for _, sm := range stored {
		// Deliver each stored message
		if err := client.Send(sm.Frame); err != nil {
			log.Printf("relay: failed to flush stored message to %s: %v", client.UUID[:8], err)
			continue
		}
		log.Printf("relay: flushed stored message to %s (from %s)", client.UUID[:8], sm.SenderUUID[:8])

		// Notify original sender if online
		rs.clientsMu.RLock()
		sender, senderOnline := rs.clients[sm.SenderUUID]
		rs.clientsMu.RUnlock()

		if senderOnline {
			if err := sender.Send(protocol.RelayFrame{
				Type:      "delivered",
				MessageID: sm.Frame.MessageID,
			}); err != nil {
				log.Printf("relay: failed to send delivery receipt to %s: %v", sm.SenderUUID[:8], err)
			}
		}
	}

	if len(stored) > 0 {
		log.Printf("relay: flushed %d stored messages to %s", len(stored), client.Username)
	}
}

// UnregisterClient removes a client from the active connections.
func (rs *RelayServer) UnregisterClient(uuid string) {
	rs.clientsMu.Lock()
	delete(rs.clients, uuid)
	rs.clientsMu.Unlock()
}

// HandleRelay routes a message or stores it for offline recipients.
func (rs *RelayServer) HandleRelay(sender *ClientConn, frame protocol.RelayFrame) {
	recipientUUID := frame.Recipient

	rs.clientsMu.RLock()
	target, online := rs.clients[recipientUUID]
	rs.clientsMu.RUnlock()

	if online {
		err := target.Send(protocol.RelayFrame{
			Type:    "msg",
			UUID:    sender.UUID,
			Message: frame.Message,
		})
		if err != nil {
			log.Printf("relay: failed to forward message from %s to %s: %v", sender.UUID[:8], recipientUUID[:8], err)
			_ = sender.Send(protocol.RelayFrame{Type: "offline", Recipient: recipientUUID})
		}
	} else {
		// Extract message ID for ack
		messageID := ""
		if frame.Message != nil {
			var inner struct {
				Message *struct {
					ID string `json:"id"`
				} `json:"message,omitempty"`
			}
			if err := json.Unmarshal(frame.Message, &inner); err == nil && inner.Message != nil {
				messageID = inner.Message.ID
			}
		}

		// Store for offline recipient
		rs.storeMu.Lock()
		rs.messageStore[recipientUUID] = append(rs.messageStore[recipientUUID], StoredMessage{
			SenderUUID:     sender.UUID,
			SenderUsername: sender.Username,
			Frame: protocol.RelayFrame{
				Type:      "msg",
				UUID:      sender.UUID,
				Message:   frame.Message,
				MessageID: messageID,
			},
			StoredAt: time.Now(),
		})
		rs.storeMu.Unlock()

		// Acknowledge storage to sender
		if err := sender.Send(protocol.RelayFrame{
			Type:      "stored",
			MessageID: messageID,
		}); err != nil {
			log.Printf("relay: failed to send stored ack to %s: %v", sender.UUID[:8], err)
		}

		log.Printf("relay: stored message from %s for offline recipient %s", sender.UUID[:8], recipientUUID[:8])
	}
}

// HandleSearch processes a search request and returns matching users.
func (rs *RelayServer) HandleSearch(sender *ClientConn, frame protocol.RelayFrame) {
	query := strings.ToLower(frame.Query)
	var results []protocol.UserInfo

	rs.registryMu.RLock()
	for _, user := range rs.userRegistry {
		if query == "" || strings.Contains(strings.ToLower(user.Username), query) {
			rs.clientsMu.RLock()
			_, isOnline := rs.clients[user.UUID]
			rs.clientsMu.RUnlock()
			results = append(results, protocol.UserInfo{
				UUID:     user.UUID,
				Username: user.Username,
				Online:   isOnline,
			})
		}
	}
	rs.registryMu.RUnlock()

	if err := sender.Send(protocol.RelayFrame{Type: "search_result", Users: results}); err != nil {
		log.Printf("relay: failed to send search results to %s: %v", sender.UUID[:8], err)
	}
}

// HandleWhoOnline returns currently connected users.
func (rs *RelayServer) HandleWhoOnline(sender *ClientConn) {
	var online []protocol.UserInfo

	rs.clientsMu.RLock()
	for uuid, cc := range rs.clients {
		online = append(online, protocol.UserInfo{UUID: uuid, Username: cc.Username, Online: true})
	}
	rs.clientsMu.RUnlock()

	if err := sender.Send(protocol.RelayFrame{Type: "online_list", Users: online}); err != nil {
		log.Printf("relay: failed to send online list to %s: %v", sender.UUID[:8], err)
	}
}

// StoredCount returns the number of stored messages for a recipient (used in tests).
func (rs *RelayServer) StoredCount(recipientUUID string) int {
	rs.storeMu.RLock()
	defer rs.storeMu.RUnlock()
	return len(rs.messageStore[recipientUUID])
}

// handleClient processes frames from a single TCP connection.
func (rs *RelayServer) handleClient(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var client *ClientConn

	defer func() {
		if client != nil {
			rs.clientsMu.Lock()
			// Only delete if the active connection matches this connection instance
			if rs.clients[client.UUID] == client {
				delete(rs.clients, client.UUID)
			}
			rs.clientsMu.Unlock()
			log.Printf("Client disconnected: %s (%s)", client.Username, client.UUID[:8])
		}
	}()

	for {
		var frame protocol.RelayFrame
		err := dec.Decode(&frame)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
			}
			return
		}

		switch frame.Type {
		case "register":
			client = &ClientConn{
				UUID:     frame.UUID,
				Username: frame.Username,
				conn:     conn,
				enc:      enc,
			}
			rs.RegisterClient(client)

		case "relay":
			if client == nil {
				log.Printf("Unregistered client attempted to relay messages")
				return
			}
			rs.HandleRelay(client, frame)

		case "search":
			if client == nil {
				log.Printf("Unregistered client attempted to search")
				return
			}
			rs.HandleSearch(client, frame)

		case "who_online":
			if client == nil {
				log.Printf("Unregistered client attempted who_online")
				return
			}
			rs.HandleWhoOnline(client)

		case "ping":
			if client != nil {
				if err := client.Send(protocol.RelayFrame{Type: "pong"}); err != nil {
					log.Printf("relay: failed to send pong to %s: %v", client.UUID[:8], err)
				}
			} else {
				if err := enc.Encode(protocol.RelayFrame{Type: "pong"}); err != nil {
					log.Printf("relay: failed to send pong to unregistered client: %v", err)
				}
			}
		}
	}
}

func main() {
	portFlag := flag.Int("port", 55558, "Port to run the relay server on")
	flag.Parse()

	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", *portFlag))
	if err != nil {
		log.Fatalf("Relay Server error: failed to listen on port %d: %v", *portFlag, err)
	}
	defer listener.Close()

	rs := NewRelayServer()

	log.Printf("TermTalk Relay Server v0.3.0 running on port %d...", *portFlag)
	log.Printf("Features: store-and-forward, user registry, search, presence")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Connection accept error: %v", err)
			continue
		}
		go rs.handleClient(conn)
	}
}
