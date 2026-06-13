package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// RelayFrame represents the message wrapper used by the relay server.
type RelayFrame struct {
	Type      string          `json:"type"`                // "register", "relay", "msg", "offline", "ping"
	UUID      string          `json:"uuid,omitempty"`      // Client registration UUID
	Username  string          `json:"username,omitempty"`  // Client registration Username
	Recipient string          `json:"recipient,omitempty"`  // Target Recipient UUID
	Message   json.RawMessage `json:"message,omitempty"`   // Nested Message payload
}

type ClientConn struct {
	UUID     string
	Username string
	conn     net.Conn
	enc      *json.Encoder
}

var (
	clients   = make(map[string]*ClientConn)
	clientsMu sync.RWMutex
)

func main() {
	portFlag := flag.Int("port", 55558, "Port to run the relay server on")
	flag.Parse()

	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", *portFlag))
	if err != nil {
		log.Fatalf("Relay Server error: failed to listen on port %d: %v", *portFlag, err)
	}
	defer listener.Close()

	log.Printf("TermTalk Relay Server running on port %d...", *portFlag)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Connection accept error: %v", err)
			continue
		}
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var client *ClientConn

	defer func() {
		if client != nil {
			clientsMu.Lock()
			delete(clients, client.UUID)
			clientsMu.Unlock()
			log.Printf("Client disconnected: %s (%s)", client.Username, client.UUID[:8])
		}
	}()

	for {
		var frame RelayFrame
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
			clientsMu.Lock()
			// Close old connection if peer registers again
			if old, exists := clients[client.UUID]; exists {
				old.conn.Close()
			}
			clients[client.UUID] = client
			clientsMu.Unlock()
			log.Printf("Client registered: %s (%s) from %s", client.Username, client.UUID[:8], conn.RemoteAddr())

			// Respond with success ack
			_ = enc.Encode(RelayFrame{Type: "registered"})

		case "relay":
			if client == nil {
				log.Printf("Unregistered client attempted to relay messages")
				return
			}

			recipientUUID := frame.Recipient

			clientsMu.RLock()
			target, online := clients[recipientUUID]
			clientsMu.RUnlock()

			if online {
				// Forward message frame directly
				err := target.enc.Encode(RelayFrame{
					Type:    "msg",
					UUID:    client.UUID,
					Message: frame.Message,
				})
				if err != nil {
					log.Printf("Failed to forward message from %s to %s: %v", client.UUID[:8], recipientUUID[:8], err)
					_ = enc.Encode(RelayFrame{Type: "offline", Recipient: recipientUUID})
				}
			} else {
				// Recipient is offline, notify sender
				_ = enc.Encode(RelayFrame{Type: "offline", Recipient: recipientUUID})
			}

		case "ping":
			_ = enc.Encode(RelayFrame{Type: "pong"})
		}
	}
}
