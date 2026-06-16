package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"termtalk/internal/protocol"
)

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
			// Only delete if the active connection matches this connection instance
			if clients[client.UUID] == client {
				delete(clients, client.UUID)
			}
			clientsMu.Unlock()
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
			clientsMu.Lock()
			// Close old connection if peer registers again
			if old, exists := clients[client.UUID]; exists {
				old.conn.Close()
			}
			clients[client.UUID] = client
			clientsMu.Unlock()
			log.Printf("Client registered: %s (%s) from %s", client.Username, client.UUID[:8], conn.RemoteAddr())

			// Respond with success ack
			if err := client.Send(protocol.RelayFrame{Type: "registered"}); err != nil {
				log.Printf("relay: failed to send registered ack to %s: %v", client.UUID[:8], err)
			}

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
				err := target.Send(protocol.RelayFrame{
					Type:    "msg",
					UUID:    client.UUID,
					Message: frame.Message,
				})
				if err != nil {
					log.Printf("relay: failed to forward message from %s to %s: %v", client.UUID[:8], recipientUUID[:8], err)
					if err := client.Send(protocol.RelayFrame{Type: "offline", Recipient: recipientUUID}); err != nil {
						log.Printf("relay: failed to send offline notification to %s: %v", client.UUID[:8], err)
					}
				}
			} else {
				// Recipient is offline, notify sender
				if err := client.Send(protocol.RelayFrame{Type: "offline", Recipient: recipientUUID}); err != nil {
					log.Printf("relay: failed to send offline notification to %s: %v", client.UUID[:8], err)
				}
			}

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
