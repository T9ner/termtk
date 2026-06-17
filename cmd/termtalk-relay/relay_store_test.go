package main

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"termtalk/internal/protocol"
)

// ─── Test helpers ──────────────────────────────────────────────────────────────

// testClient wraps a ClientConn plus a local net.Conn end for reading frames.
type testClient struct {
	cc   *ClientConn
	conn net.Conn      // the "client-side" end for reading/writing
	dec  *json.Decoder // persistent decoder for the client-side connection
}

// makeTestClient creates a ClientConn backed by an in-memory TCP connection.
// Uses a local TCP listener so writes don't block (unlike net.Pipe).
func makeTestClient(t *testing.T, uuid, username string) *testClient {
	t.Helper()

	// Start a one-shot listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("makeTestClient: listen: %v", err)
	}

	var serverConn net.Conn
	connReady := make(chan struct{})
	go func() {
		var err error
		serverConn, err = l.Accept()
		if err != nil {
			return
		}
		close(connReady)
	}()

	clientConn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		l.Close()
		t.Fatalf("makeTestClient: dial: %v", err)
	}
	<-connReady
	l.Close()

	cc := &ClientConn{
		UUID:     uuid,
		Username: username,
		conn:     serverConn,
		enc:      json.NewEncoder(serverConn),
	}

	return &testClient{cc: cc, conn: clientConn, dec: json.NewDecoder(clientConn)}
}

func (tc *testClient) Close() {
	tc.cc.conn.Close()
	tc.conn.Close()
}

// readFrame reads a single RelayFrame from the client-side connection.
func (tc *testClient) readFrame(t *testing.T, timeout time.Duration) protocol.RelayFrame {
	t.Helper()
	tc.conn.SetReadDeadline(time.Now().Add(timeout))
	var frame protocol.RelayFrame
	if err := tc.dec.Decode(&frame); err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	tc.conn.SetReadDeadline(time.Time{})
	return frame
}

// ─── Store-and-Forward Tests ───────────────────────────────────────────────────

func TestStoreMessageForOfflineRecipient(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()

	rs.RegisterClient(alice.cc)
	ack := alice.readFrame(t, time.Second)
	if ack.Type != "registered" {
		t.Fatalf("expected 'registered' ack, got '%s'", ack.Type)
	}

	// Alice sends a message to offline Bob via relay
	innerPayload, _ := json.Marshal(map[string]interface{}{
		"type": "msg",
		"message": map[string]interface{}{
			"id":             "msg-hash-001",
			"sender_uuid":    "alice-uuid",
			"recipient_uuid": "bob-uuid",
			"content":        "Hello offline Bob!",
			"timestamp":      time.Now().Format(time.RFC3339Nano),
			"status":         "queued",
		},
	})

	rs.HandleRelay(alice.cc, protocol.RelayFrame{
		Type:      "relay",
		Recipient: "bob-uuid",
		Message:   innerPayload,
	})

	stored := alice.readFrame(t, time.Second)
	if stored.Type != "stored" {
		t.Fatalf("expected 'stored' ack, got '%s'", stored.Type)
	}
	if stored.MessageID != "msg-hash-001" {
		t.Errorf("expected message_id 'msg-hash-001', got '%s'", stored.MessageID)
	}

	if rs.StoredCount("bob-uuid") != 1 {
		t.Errorf("expected 1 stored message for bob, got %d", rs.StoredCount("bob-uuid"))
	}
}

func TestFlushStoredMessagesOnReconnect(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()

	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second) // registered ack

	// Alice sends to offline Bob
	innerPayload, _ := json.Marshal(map[string]interface{}{
		"type": "msg",
		"message": map[string]interface{}{
			"id":             "msg-hash-002",
			"sender_uuid":    "alice-uuid",
			"recipient_uuid": "bob-uuid",
			"content":        "Stored message for Bob",
			"timestamp":      time.Now().Format(time.RFC3339Nano),
			"status":         "queued",
		},
	})

	rs.HandleRelay(alice.cc, protocol.RelayFrame{
		Type:      "relay",
		Recipient: "bob-uuid",
		Message:   innerPayload,
	})
	_ = alice.readFrame(t, time.Second) // stored ack

	// Bob connects — stored messages should flush
	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()

	rs.RegisterClient(bob.cc)

	bobAck := bob.readFrame(t, time.Second)
	if bobAck.Type != "registered" {
		t.Fatalf("expected 'registered' ack for bob, got '%s'", bobAck.Type)
	}

	flushed := bob.readFrame(t, time.Second)
	if flushed.Type != "msg" {
		t.Fatalf("expected flushed 'msg' frame, got '%s'", flushed.Type)
	}
	if flushed.UUID != "alice-uuid" {
		t.Errorf("expected sender UUID 'alice-uuid', got '%s'", flushed.UUID)
	}

	if rs.StoredCount("bob-uuid") != 0 {
		t.Errorf("expected 0 stored messages after flush, got %d", rs.StoredCount("bob-uuid"))
	}
}

func TestDeliveryReceiptSentToSender(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()

	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second) // registered ack

	// Alice sends to offline Bob
	innerPayload, _ := json.Marshal(map[string]interface{}{
		"type": "msg",
		"message": map[string]interface{}{
			"id":             "msg-hash-003",
			"sender_uuid":    "alice-uuid",
			"recipient_uuid": "bob-uuid",
			"content":        "Please deliver this",
			"timestamp":      time.Now().Format(time.RFC3339Nano),
			"status":         "queued",
		},
	})

	rs.HandleRelay(alice.cc, protocol.RelayFrame{
		Type:      "relay",
		Recipient: "bob-uuid",
		Message:   innerPayload,
	})
	_ = alice.readFrame(t, time.Second) // stored ack

	// Bob connects — triggers flush + delivery receipt to Alice
	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()

	rs.RegisterClient(bob.cc)
	_ = bob.readFrame(t, time.Second) // registered ack
	_ = bob.readFrame(t, time.Second) // flushed msg

	// Alice should receive a "delivered" receipt
	delivered := alice.readFrame(t, time.Second)
	if delivered.Type != "delivered" {
		t.Fatalf("expected 'delivered' receipt, got '%s'", delivered.Type)
	}
	if delivered.MessageID != "msg-hash-003" {
		t.Errorf("expected message_id 'msg-hash-003', got '%s'", delivered.MessageID)
	}
}

func TestNoStoreWhenRecipientOnline(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()

	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()

	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second) // registered ack

	rs.RegisterClient(bob.cc)
	_ = bob.readFrame(t, time.Second) // registered ack

	// Alice sends to online Bob
	innerPayload, _ := json.Marshal(map[string]interface{}{
		"type": "msg",
		"message": map[string]interface{}{
			"id":             "msg-hash-004",
			"sender_uuid":    "alice-uuid",
			"recipient_uuid": "bob-uuid",
			"content":        "Direct delivery",
			"timestamp":      time.Now().Format(time.RFC3339Nano),
			"status":         "queued",
		},
	})

	rs.HandleRelay(alice.cc, protocol.RelayFrame{
		Type:      "relay",
		Recipient: "bob-uuid",
		Message:   innerPayload,
	})

	msg := bob.readFrame(t, time.Second)
	if msg.Type != "msg" {
		t.Fatalf("expected 'msg' frame, got '%s'", msg.Type)
	}

	if rs.StoredCount("bob-uuid") != 0 {
		t.Errorf("expected 0 stored messages when online, got %d", rs.StoredCount("bob-uuid"))
	}
}

// ─── User Registry & Search Tests ──────────────────────────────────────────────

func TestSearchFindsRegisteredUser(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice_wonder")
	defer alice.Close()
	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second) // registered ack

	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()
	rs.RegisterClient(bob.cc)
	_ = bob.readFrame(t, time.Second) // registered ack

	// Bob searches for "alice"
	rs.HandleSearch(bob.cc, protocol.RelayFrame{
		Type:  "search",
		Query: "alice",
	})

	result := bob.readFrame(t, time.Second)
	if result.Type != "search_result" {
		t.Fatalf("expected 'search_result', got '%s'", result.Type)
	}
	if len(result.Users) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(result.Users))
	}
	if result.Users[0].UUID != "alice-uuid" {
		t.Errorf("expected UUID 'alice-uuid', got '%s'", result.Users[0].UUID)
	}
	if result.Users[0].Username != "alice_wonder" {
		t.Errorf("expected Username 'alice_wonder', got '%s'", result.Users[0].Username)
	}
}

func TestSearchReturnsOnlineStatus(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()
	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second) // registered ack

	// Add charlie to registry but NOT to clients (simulates offline)
	rs.registryMu.Lock()
	rs.userRegistry["charlie-uuid"] = RegisteredUser{
		UUID:     "charlie-uuid",
		Username: "charlie",
		LastSeen: time.Now(),
	}
	rs.registryMu.Unlock()

	rs.HandleSearch(alice.cc, protocol.RelayFrame{
		Type:  "search",
		Query: "",
	})

	result := alice.readFrame(t, time.Second)
	if result.Type != "search_result" {
		t.Fatalf("expected 'search_result', got '%s'", result.Type)
	}
	if len(result.Users) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Users))
	}

	onlineCount := 0
	offlineCount := 0
	for _, u := range result.Users {
		if u.Online {
			onlineCount++
		} else {
			offlineCount++
		}
	}
	if onlineCount != 1 {
		t.Errorf("expected 1 online user, got %d", onlineCount)
	}
	if offlineCount != 1 {
		t.Errorf("expected 1 offline user, got %d", offlineCount)
	}
}

func TestWhoOnlineReturnsConnectedUsers(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()

	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()

	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second)

	rs.RegisterClient(bob.cc)
	_ = bob.readFrame(t, time.Second)

	rs.HandleWhoOnline(alice.cc)

	result := alice.readFrame(t, time.Second)
	if result.Type != "online_list" {
		t.Fatalf("expected 'online_list', got '%s'", result.Type)
	}
	if len(result.Users) != 2 {
		t.Errorf("expected 2 online users, got %d", len(result.Users))
	}
	for _, u := range result.Users {
		if !u.Online {
			t.Errorf("all users in online_list should have Online=true, got false for %s", u.Username)
		}
	}
}

func TestSearchEmptyQueryReturnsAll(t *testing.T) {
	rs := NewRelayServer(nil)

	alice := makeTestClient(t, "alice-uuid", "alice")
	defer alice.Close()
	rs.RegisterClient(alice.cc)
	_ = alice.readFrame(t, time.Second)

	bob := makeTestClient(t, "bob-uuid", "bob")
	defer bob.Close()
	rs.RegisterClient(bob.cc)
	_ = bob.readFrame(t, time.Second)

	charlie := makeTestClient(t, "charlie-uuid", "charlie")
	defer charlie.Close()
	rs.RegisterClient(charlie.cc)
	_ = charlie.readFrame(t, time.Second)

	rs.HandleSearch(alice.cc, protocol.RelayFrame{
		Type:  "search",
		Query: "",
	})

	result := alice.readFrame(t, time.Second)
	if result.Type != "search_result" {
		t.Fatalf("expected 'search_result', got '%s'", result.Type)
	}
	if len(result.Users) != 3 {
		t.Errorf("expected 3 users for empty query, got %d", len(result.Users))
	}
}
