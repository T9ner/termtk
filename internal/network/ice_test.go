package network

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v4"
)

// TestICELoopbackNegotiation creates two ICE agents (offerer + answerer)
// that negotiate directly via loopback, bypassing the relay signaling path.
// This verifies ICE connectivity check logic and candidate exchange.
func TestICELoopbackNegotiation(t *testing.T) {
	// Create two ICE agents with host candidates only (loopback)
	offerer, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	})
	if err != nil {
		t.Fatalf("failed to create offerer agent: %v", err)
	}
	defer offerer.Close()

	answerer, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	})
	if err != nil {
		t.Fatalf("failed to create answerer agent: %v", err)
	}
	defer answerer.Close()

	// Gather candidates from both agents
	offererCandidates := gatherTestCandidates(t, offerer)
	answererCandidates := gatherTestCandidates(t, answerer)

	if len(offererCandidates) == 0 {
		t.Fatal("offerer gathered no candidates")
	}
	if len(answererCandidates) == 0 {
		t.Fatal("answerer gathered no candidates")
	}

	t.Logf("offerer gathered %d candidates, answerer gathered %d candidates",
		len(offererCandidates), len(answererCandidates))

	// Get credentials
	oUfrag, oPwd, err := offerer.GetLocalUserCredentials()
	if err != nil {
		t.Fatalf("offerer credentials: %v", err)
	}
	aUfrag, aPwd, err := answerer.GetLocalUserCredentials()
	if err != nil {
		t.Fatalf("answerer credentials: %v", err)
	}

	// Add remote candidates to each side
	for _, cs := range answererCandidates {
		c, err := ice.UnmarshalCandidate(cs)
		if err != nil {
			t.Fatalf("unmarshal answerer candidate: %v", err)
		}
		if err := offerer.AddRemoteCandidate(c); err != nil {
			t.Fatalf("offerer add remote candidate: %v", err)
		}
	}
	for _, cs := range offererCandidates {
		c, err := ice.UnmarshalCandidate(cs)
		if err != nil {
			t.Fatalf("unmarshal offerer candidate: %v", err)
		}
		if err := answerer.AddRemoteCandidate(c); err != nil {
			t.Fatalf("answerer add remote candidate: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Dial + Accept concurrently
	var wg sync.WaitGroup
	var dialErr, acceptErr error
	var offererConn, answererConn *ice.Conn

	wg.Add(2)
	go func() {
		defer wg.Done()
		offererConn, dialErr = offerer.Dial(ctx, aUfrag, aPwd)
	}()
	go func() {
		defer wg.Done()
		answererConn, acceptErr = answerer.Accept(ctx, oUfrag, oPwd)
	}()
	wg.Wait()

	if dialErr != nil {
		t.Fatalf("offerer dial failed: %v", dialErr)
	}
	if acceptErr != nil {
		t.Fatalf("answerer accept failed: %v", acceptErr)
	}

	defer offererConn.Close()
	defer answererConn.Close()

	t.Log("ICE connectivity check succeeded — direct connection established")

	// Send a test message from offerer to answerer
	testFrame := Frame{
		Type: "msg",
	}
	enc := json.NewEncoder(offererConn)
	if err := enc.Encode(testFrame); err != nil {
		t.Fatalf("offerer send failed: %v", err)
	}

	dec := json.NewDecoder(answererConn)
	var received Frame
	if err := dec.Decode(&received); err != nil {
		t.Fatalf("answerer receive failed: %v", err)
	}

	if received.Type != "msg" {
		t.Errorf("expected frame type %q, got %q", "msg", received.Type)
	}

	t.Log("JSON frame sent and received over ICE connection successfully")
}

// TestICESignalMarshal verifies ICESignal serialization roundtrip.
func TestICESignalMarshal(t *testing.T) {
	signal := ICESignal{
		Ufrag: "testufrag",
		Pwd:   "testpwd123",
		Candidates: []string{
			"candidate:1 1 UDP 2130706431 192.168.1.1 12345 typ host",
			"candidate:2 1 UDP 1694498815 10.0.0.1 54321 typ srflx",
		},
	}

	data, err := json.Marshal(signal)
	if err != nil {
		t.Fatalf("marshal ICESignal: %v", err)
	}

	var decoded ICESignal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal ICESignal: %v", err)
	}

	if decoded.Ufrag != signal.Ufrag {
		t.Errorf("ufrag mismatch: got %q want %q", decoded.Ufrag, signal.Ufrag)
	}
	if decoded.Pwd != signal.Pwd {
		t.Errorf("pwd mismatch: got %q want %q", decoded.Pwd, signal.Pwd)
	}
	if len(decoded.Candidates) != len(signal.Candidates) {
		t.Errorf("candidate count mismatch: got %d want %d", len(decoded.Candidates), len(signal.Candidates))
	}
}

// TestICESignalInFrame verifies ICESignal round-trips through Frame marshaling.
func TestICESignalInFrame(t *testing.T) {
	signal := ICESignal{
		Ufrag:      "u1",
		Pwd:        "p1",
		Candidates: []string{"candidate:1 1 UDP 2130706431 127.0.0.1 5000 typ host"},
	}

	signalBytes, err := json.Marshal(signal)
	if err != nil {
		t.Fatalf("marshal signal: %v", err)
	}

	frame := Frame{
		Type:      "ice_offer",
		ICESignal: signalBytes,
	}

	frameBytes, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}

	var decoded Frame
	if err := json.Unmarshal(frameBytes, &decoded); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}

	if decoded.Type != "ice_offer" {
		t.Errorf("frame type: got %q want %q", decoded.Type, "ice_offer")
	}

	parsed, err := parseICESignal(decoded)
	if err != nil {
		t.Fatalf("parseICESignal: %v", err)
	}

	if parsed.Ufrag != "u1" {
		t.Errorf("ufrag: got %q want %q", parsed.Ufrag, "u1")
	}
	if len(parsed.Candidates) != 1 {
		t.Errorf("candidates: got %d want 1", len(parsed.Candidates))
	}
}

// gatherTestCandidates collects ICE candidates from an agent for testing.
func gatherTestCandidates(t *testing.T, agent *ice.Agent) []string {
	t.Helper()

	candidatesCh := make(chan string, 32)
	doneCh := make(chan struct{})
	var once sync.Once

	if err := agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			once.Do(func() { close(doneCh) })
			return
		}
		select {
		case candidatesCh <- c.Marshal():
		default:
		}
	}); err != nil {
		t.Fatalf("OnCandidate: %v", err)
	}

	if err := agent.GatherCandidates(); err != nil {
		t.Fatalf("GatherCandidates: %v", err)
	}

	var candidates []string
	timeout := time.After(5 * time.Second)

	for {
		select {
		case c := <-candidatesCh:
			candidates = append(candidates, c)
		case <-doneCh:
			for {
				select {
				case c := <-candidatesCh:
					candidates = append(candidates, c)
				default:
					return candidates
				}
			}
		case <-timeout:
			return candidates
		}
	}
}
