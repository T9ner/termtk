package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"termtalk/internal/db"
	"termtalk/internal/protocol"
)

// MockRelayServer represents an in-memory relay server for integration tests.
type MockRelayServer struct {
	listener net.Listener
	clients  map[string]*mockClientConn
	mu       sync.Mutex
	stopChan chan struct{}
	wg       sync.WaitGroup
}

type mockClientConn struct {
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

func (mcc *mockClientConn) Send(frame protocol.RelayFrame) error {
	mcc.mu.Lock()
	defer mcc.mu.Unlock()
	return mcc.enc.Encode(frame)
}

// StartMockRelay boots a mock relay server listening on the specified address.
func StartMockRelay(t *testing.T, addr string) *MockRelayServer {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to start mock relay: %v", err)
	}

	mr := &MockRelayServer{
		listener: l,
		clients:  make(map[string]*mockClientConn),
		stopChan: make(chan struct{}),
	}

	mr.wg.Add(1)
	go mr.acceptLoop()

	return mr
}

// Addr returns the listener's actual network address.
func (mr *MockRelayServer) Addr() string {
	return mr.listener.Addr().String()
}

// Stop stops the mock relay server.
func (mr *MockRelayServer) Stop() {
	close(mr.stopChan)
	mr.listener.Close()
	mr.mu.Lock()
	for _, c := range mr.clients {
		c.conn.Close()
	}
	mr.mu.Unlock()
	mr.wg.Wait()
}

func (mr *MockRelayServer) acceptLoop() {
	defer mr.wg.Done()
	for {
		conn, err := mr.listener.Accept()
		if err != nil {
			return
		}
		go mr.handleClient(conn)
	}
}

func (mr *MockRelayServer) handleClient(conn net.Conn) {
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	var client *mockClientConn

	defer func() {
		conn.Close()
		if client != nil {
			mr.mu.Lock()
			for u, cc := range mr.clients {
				if cc == client {
					delete(mr.clients, u)
				}
			}
			mr.mu.Unlock()
		}
	}()

	for {
		var frame protocol.RelayFrame
		if err := dec.Decode(&frame); err != nil {
			return
		}

		switch frame.Type {
		case "register":
			client = &mockClientConn{
				conn: conn,
				enc:  enc,
			}
			mr.mu.Lock()
			if old, exists := mr.clients[frame.UUID]; exists {
				old.conn.Close()
			}
			mr.clients[frame.UUID] = client
			mr.mu.Unlock()
			_ = client.Send(protocol.RelayFrame{Type: "registered"})

		case "relay":
			mr.mu.Lock()
			target, online := mr.clients[frame.Recipient]
			mr.mu.Unlock()

			if online {
				senderUUID := ""
				mr.mu.Lock()
				for u, cc := range mr.clients {
					if cc == client {
						senderUUID = u
						break
					}
				}
				mr.mu.Unlock()

				_ = target.Send(protocol.RelayFrame{
					Type:    "msg",
					UUID:    senderUUID,
					Message: frame.Message,
				})
			} else {
				if client != nil {
					_ = client.Send(protocol.RelayFrame{Type: "offline", Recipient: frame.Recipient})
				} else {
					_ = enc.Encode(protocol.RelayFrame{Type: "offline", Recipient: frame.Recipient})
				}
			}
		case "ping":
			if client != nil {
				_ = client.Send(protocol.RelayFrame{Type: "pong"})
			} else {
				_ = enc.Encode(protocol.RelayFrame{Type: "pong"})
			}
		}
	}
}

func TestSyncManagerViaRelay(t *testing.T) {
	// Start mock relay on OS-assigned port
	mr := StartMockRelay(t, "127.0.0.1:0")
	defer mr.Stop()
	relayAddr := mr.Addr()

	// 1. Create DB and SyncManager for Alice (port 0 = OS-assigned)
	aliceDB, aliceCleanup := createTempDB(t, "alice_relay")
	defer aliceCleanup()

	aliceProfile := &db.Profile{UUID: "alice-uuid", Username: "alice"}
	_ = aliceDB.SaveProfile(aliceProfile)

	aliceSync := NewSyncManager(aliceProfile.UUID, aliceProfile.Username, 0, aliceDB)
	aliceSync.SetRelayAddr(relayAddr)
	_ = aliceSync.Start(context.Background())
	defer aliceSync.Stop()

	// 2. Create DB and SyncManager for Bob (port 0 = OS-assigned)
	bobDB, bobCleanup := createTempDB(t, "bob_relay")
	defer bobCleanup()

	bobProfile := &db.Profile{UUID: "bob-uuid", Username: "bob"}
	_ = bobDB.SaveProfile(bobProfile)

	bobSync := NewSyncManager(bobProfile.UUID, bobProfile.Username, 0, bobDB)
	bobSync.SetRelayAddr(relayAddr)
	_ = bobSync.Start(context.Background())
	defer bobSync.Stop()

	// Register Bob in Alice's contact list (as offline/relay target)
	_ = aliceDB.UpsertContact(&db.Contact{
		UUID:     bobProfile.UUID,
		Username: bobProfile.Username,
		IP:       "offline",
		Port:     0,
		LastSeen: time.Now(),
	})

	// Wait for registration on relay
	for i := 0; i < 50; i++ {
		if aliceSync.IsRelayOnline() && bobSync.IsRelayOnline() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !aliceSync.IsRelayOnline() || !bobSync.IsRelayOnline() {
		t.Fatalf("alice or bob failed to connect to relay in time")
	}

	// Bind Bob callback to wait for relayed message
	messageReceived := make(chan *db.Message, 1)
	bobSync.OnMsgRecv = func(msg *db.Message) {
		if msg.Status == string(db.StatusSynced) {
			messageReceived <- msg
		}
	}

	// 3. Send Message from Alice to Bob (routes through relay)
	err := aliceSync.SendMessage(bobProfile.UUID, "Hello via relay Bob!")
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Verify Bob received it
	select {
	case msg := <-messageReceived:
		if msg.Content != "Hello via relay Bob!" {
			t.Errorf("expected content 'Hello via relay Bob!', got '%s'", msg.Content)
		}
		if msg.Sender != aliceProfile.UUID {
			t.Errorf("expected sender '%s', got '%s'", aliceProfile.UUID, msg.Sender)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("bob timed out waiting for relayed message")
	}

	// Suppress unused import warning
	_ = fmt.Sprintf
}
