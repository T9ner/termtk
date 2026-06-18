package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/stun/v3"
)

// ICESignal carries ICE negotiation data through the relay as inner Frame payloads.
type ICESignal struct {
	Ufrag      string   `json:"ufrag"`
	Pwd        string   `json:"pwd"`
	Candidates []string `json:"candidates"` // Marshaled ICE candidates
}

// Default STUN servers for ICE candidate gathering.
var defaultSTUNServers = []string{
	"stun:stun.l.google.com:19302",
	"stun:stun1.l.google.com:19302",
}

// iceNegotiation tracks a pending ICE negotiation with a peer.
type iceNegotiation struct {
	agent    *ice.Agent
	answerCh chan ICESignal // receives the peer's answer
	cancel   context.CancelFunc
	once     sync.Once // CE-003: safe channel close
}

// ICEManager handles ICE-based NAT hole punching for direct peer connections.
// It uses the relay as a signaling channel and registers successful ICE
// connections in SyncManager.activeConn so the existing JSON sync protocol
// runs on top of them.
type ICEManager struct {
	sm *SyncManager // back-pointer to owning SyncManager

	mu           sync.Mutex
	negotiations map[string]*iceNegotiation // peerUUID -> pending negotiation

	stunServers []*stun.URI
	timeout     time.Duration

	stopChan chan struct{}
	stopOnce sync.Once // CE-003: safe channel close
}

// NewICEManager creates an ICEManager attached to the given SyncManager.
func NewICEManager(sm *SyncManager) *ICEManager {
	urls := make([]*stun.URI, 0, len(defaultSTUNServers))
	for _, s := range defaultSTUNServers {
		u, err := stun.ParseURI(s)
		if err != nil {
			log.Printf("ice: failed to parse STUN URI %q: %v", s, err)
			continue
		}
		urls = append(urls, u)
	}

	return &ICEManager{
		sm:           sm,
		negotiations: make(map[string]*iceNegotiation),
		stunServers:  urls,
		timeout:      10 * time.Second,
		stopChan:     make(chan struct{}),
	}
}

// Close shuts down the ICE manager and all pending negotiations.
func (im *ICEManager) Close() {
	im.stopOnce.Do(func() {
		close(im.stopChan)
	})

	im.mu.Lock()
	defer im.mu.Unlock()

	for peerUUID, neg := range im.negotiations {
		neg.cancel()
		if neg.agent != nil {
			_ = neg.agent.Close()
		}
		neg.once.Do(func() {
			close(neg.answerCh)
		})
		delete(im.negotiations, peerUUID)
	}
}

// InitiateConnection starts an ICE negotiation as the controlling (offering) agent.
// It gathers local candidates, sends an ice_offer through the relay, waits for
// the peer's ice_answer, and if connectivity checks succeed, registers the
// resulting net.Conn as a PeerConnection in SyncManager.
func (im *ICEManager) InitiateConnection(peerUUID string) {
	select {
	case <-im.stopChan:
		return
	default:
	}

	im.mu.Lock()
	if _, exists := im.negotiations[peerUUID]; exists {
		im.mu.Unlock()
		return // negotiation already in progress
	}
	// Reserve slot to prevent duplicate goroutine spawns (TOCTOU fix)
	im.negotiations[peerUUID] = &iceNegotiation{}
	im.mu.Unlock()

	go im.doInitiate(peerUUID)
}

func (im *ICEManager) doInitiate(peerUUID string) {
	ctx, cancel := context.WithTimeout(context.Background(), im.timeout)
	defer cancel()

	agent, err := im.createAgent()
	if err != nil {
		log.Printf("ice: failed to create agent for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	neg := &iceNegotiation{
		agent:    agent,
		answerCh: make(chan ICESignal, 1),
		cancel:   cancel,
	}

	im.mu.Lock()
	if _, exists := im.negotiations[peerUUID]; exists {
		im.mu.Unlock()
		_ = agent.Close()
		return
	}
	im.negotiations[peerUUID] = neg
	im.mu.Unlock()

	defer im.cleanupNegotiation(peerUUID)

	// Gather local candidates
	localCandidates, err := im.gatherCandidates(ctx, agent)
	if err != nil {
		log.Printf("ice: candidate gathering failed for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	localUfrag, localPwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Printf("ice: failed to get local credentials for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	// Send ice_offer through relay
	signal := ICESignal{
		Ufrag:      localUfrag,
		Pwd:        localPwd,
		Candidates: localCandidates,
	}

	if err := im.sendICEFrame(peerUUID, "ice_offer", signal); err != nil {
		log.Printf("ice: failed to send offer to %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	log.Printf("ice: sent offer to %s (%d candidates)", im.shortUUID(peerUUID), len(localCandidates))

	// Wait for answer
	var answer ICESignal
	select {
	case answer = <-neg.answerCh:
	case <-ctx.Done():
		log.Printf("ice: offer to %s timed out", im.shortUUID(peerUUID))
		return
	case <-im.stopChan:
		return
	}

	// Add remote candidates and start connectivity checks
	conn, err := im.completeConnection(ctx, agent, answer, true)
	if err != nil {
		log.Printf("ice: connectivity check failed with %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	log.Printf("ice: direct connection established with %s", im.shortUUID(peerUUID))
	im.registerICEConnection(peerUUID, conn)
}

// HandleOffer processes an incoming ICE offer from a peer.
// Creates an answering agent, gathers candidates, sends ice_answer,
// and completes connectivity checks.
func (im *ICEManager) HandleOffer(peerUUID string, signal ICESignal) {
	select {
	case <-im.stopChan:
		return
	default:
	}

	go im.doHandleOffer(peerUUID, signal)
}

func (im *ICEManager) doHandleOffer(peerUUID string, offer ICESignal) {
	ctx, cancel := context.WithTimeout(context.Background(), im.timeout)
	defer cancel()

	agent, err := im.createAgent()
	if err != nil {
		log.Printf("ice: failed to create answering agent for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	neg := &iceNegotiation{
		agent:    agent,
		answerCh: make(chan ICESignal, 1),
		cancel:   cancel,
	}

	im.mu.Lock()
	// If there's already a negotiation from our side, we check UUIDs to break ties
	if existing, exists := im.negotiations[peerUUID]; exists {
		// Higher UUID wins as controller — if our UUID is higher, keep our offer
		localUUID := im.sm.getLocalUUID()
		if localUUID > peerUUID {
			im.mu.Unlock()
			_ = agent.Close()
			return // Our outgoing offer takes priority
		}
		// Their offer wins, cancel our outgoing one
		existing.cancel()
		if existing.agent != nil {
			_ = existing.agent.Close()
		}
		existing.once.Do(func() {
			close(existing.answerCh)
		})
	}
	im.negotiations[peerUUID] = neg
	im.mu.Unlock()

	defer im.cleanupNegotiation(peerUUID)

	// Gather local candidates
	localCandidates, err := im.gatherCandidates(ctx, agent)
	if err != nil {
		log.Printf("ice: answerer candidate gathering failed for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	localUfrag, localPwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Printf("ice: failed to get local credentials (answerer) for %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	// Send ice_answer through relay
	answerSignal := ICESignal{
		Ufrag:      localUfrag,
		Pwd:        localPwd,
		Candidates: localCandidates,
	}

	if err := im.sendICEFrame(peerUUID, "ice_answer", answerSignal); err != nil {
		log.Printf("ice: failed to send answer to %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	log.Printf("ice: sent answer to %s (%d candidates)", im.shortUUID(peerUUID), len(localCandidates))

	// Complete connectivity checks as controlled side
	conn, err := im.completeConnection(ctx, agent, offer, false)
	if err != nil {
		log.Printf("ice: connectivity check failed (answerer) with %s: %v", im.shortUUID(peerUUID), err)
		return
	}

	log.Printf("ice: direct connection established (answerer) with %s", im.shortUUID(peerUUID))
	im.registerICEConnection(peerUUID, conn)
}

// HandleAnswer delivers an ICE answer to a pending negotiation.
func (im *ICEManager) HandleAnswer(peerUUID string, signal ICESignal) {
	im.mu.Lock()
	neg, exists := im.negotiations[peerUUID]
	im.mu.Unlock()

	if !exists {
		log.Printf("ice: received answer from %s but no pending negotiation", im.shortUUID(peerUUID))
		return
	}

	select {
	case neg.answerCh <- signal:
	default:
		log.Printf("ice: answer channel full for %s, dropping", im.shortUUID(peerUUID))
	}
}

// createAgent builds a pion/ice Agent with STUN servers configured.
func (im *ICEManager) createAgent() (*ice.Agent, error) {
	cfg := &ice.AgentConfig{
		Urls: im.stunServers,
	}

	agent, err := ice.NewAgent(cfg)
	if err != nil {
		return nil, fmt.Errorf("ice.NewAgent: %w", err)
	}

	// Log ICE state transitions
	if err := agent.OnConnectionStateChange(func(state ice.ConnectionState) {
		log.Printf("ice: connection state changed: %s", state)
	}); err != nil {
		_ = agent.Close()
		return nil, fmt.Errorf("OnConnectionStateChange: %w", err)
	}

	return agent, nil
}

// gatherCandidates runs ICE candidate gathering and returns serialized candidates.
func (im *ICEManager) gatherCandidates(ctx context.Context, agent *ice.Agent) ([]string, error) {
	candidatesCh := make(chan string, 32)
	doneCh := make(chan struct{})
	var doneOnce sync.Once // CE-003

	if err := agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			// nil signals gathering is complete
			doneOnce.Do(func() {
				close(doneCh)
			})
			return
		}
		select {
		case candidatesCh <- c.Marshal():
		default:
			// Channel full, skip this candidate
		}
	}); err != nil {
		return nil, fmt.Errorf("OnCandidate: %w", err)
	}

	if err := agent.GatherCandidates(); err != nil {
		return nil, fmt.Errorf("GatherCandidates: %w", err)
	}

	var candidates []string
	gatherTimeout := time.After(5 * time.Second)

	for {
		select {
		case c := <-candidatesCh:
			candidates = append(candidates, c)
		case <-doneCh:
			// Drain remaining
			for {
				select {
				case c := <-candidatesCh:
					candidates = append(candidates, c)
				default:
					return candidates, nil
				}
			}
		case <-gatherTimeout:
			return candidates, nil
		case <-ctx.Done():
			return candidates, ctx.Err()
		}
	}
}

// completeConnection adds remote candidates and performs connectivity checks.
// isControlling determines whether this agent dials (controlling) or accepts (controlled).
func (im *ICEManager) completeConnection(
	ctx context.Context,
	agent *ice.Agent,
	remote ICESignal,
	isControlling bool,
) (net.Conn, error) {
	// Add remote candidates
	for _, cs := range remote.Candidates {
		c, err := ice.UnmarshalCandidate(cs)
		if err != nil {
			log.Printf("ice: failed to unmarshal remote candidate: %v", err)
			continue
		}
		if err := agent.AddRemoteCandidate(c); err != nil {
			log.Printf("ice: failed to add remote candidate: %v", err)
		}
	}

	// Start connectivity checks
	var conn *ice.Conn
	var err error

	if isControlling {
		conn, err = agent.Dial(ctx, remote.Ufrag, remote.Pwd)
	} else {
		conn, err = agent.Accept(ctx, remote.Ufrag, remote.Pwd)
	}

	if err != nil {
		return nil, fmt.Errorf("ICE connectivity check: %w", err)
	}

	return conn, nil
}

// registerICEConnection wraps an ICE net.Conn as a PeerConnection and runs
// the existing sync protocol on top of it, just like a LAN TCP connection.
func (im *ICEManager) registerICEConnection(peerUUID string, conn net.Conn) {
	pc := NewPeerConnection(conn)
	pc.UUID = peerUUID

	localUUID := im.sm.getLocalUUID()
	username := im.sm.getUsername()

	// Perform handshake over the ICE connection
	err := pc.Send(Frame{
		Type:     "handshake",
		UUID:     localUUID,
		Username: username,
	})
	if err != nil {
		log.Printf("ice: handshake send failed with %s: %v", im.shortUUID(peerUUID), err)
		conn.Close()
		return
	}

	// Set a deadline for handshake response
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var resp Frame
	if err := pc.dec.Decode(&resp); err != nil || resp.Type != "handshake" {
		log.Printf("ice: handshake response failed with %s: %v", im.shortUUID(peerUUID), err)
		conn.Close()
		return
	}
	conn.SetDeadline(time.Time{}) // Clear deadline

	pc.UUID = resp.UUID
	pc.Username = resp.Username

	// Register in SyncManager's active connections
	im.sm.mu.Lock()
	if oldPc, exists := im.sm.activeConn[pc.UUID]; exists {
		oldPc.Close()
	}
	im.sm.activeConn[pc.UUID] = pc
	im.sm.mu.Unlock()

	log.Printf("ice: peer %s (%s) registered via ICE direct connection", im.shortUUID(pc.UUID), pc.Username)

	// Notify client of successful ICE direct connection
	if im.sm.OnICEStatus != nil {
		im.sm.OnICEStatus(pc.UUID, true)
	}

	// Run the standard connection handler (reads frames until disconnect)
	im.sm.wg.Add(1)
	go im.sm.handleConnection(pc)

	// Sync message history
	go im.sm.SyncHistory(pc)
}

// sendICEFrame sends an ICE signaling frame through the relay.
// Uses sendRelayFrame so the ICE data travels as an inner Frame.
func (im *ICEManager) sendICEFrame(peerUUID string, frameType string, signal ICESignal) error {
	signalBytes, err := json.Marshal(signal)
	if err != nil {
		return err
	}

	return im.sm.sendRelayFrame(peerUUID, Frame{
		Type:      frameType,
		ICESignal: signalBytes,
	})
}

// cleanupNegotiation removes a negotiation entry and closes the agent.
func (im *ICEManager) cleanupNegotiation(peerUUID string) {
	im.mu.Lock()
	defer im.mu.Unlock()

	neg, exists := im.negotiations[peerUUID]
	if !exists {
		return
	}

	neg.cancel()
	if neg.agent != nil {
		_ = neg.agent.Close()
	}
	neg.once.Do(func() {
		close(neg.answerCh)
	})
	delete(im.negotiations, peerUUID)
}

// shortUUID returns the first 8 characters of a UUID for logging.
func (im *ICEManager) shortUUID(uuid string) string {
	if len(uuid) >= 8 {
		return uuid[:8]
	}
	return uuid
}

// parseICESignal extracts an ICESignal from a Frame's ICESignal field.
func parseICESignal(f Frame) (ICESignal, error) {
	var signal ICESignal
	if f.ICESignal == nil {
		return signal, fmt.Errorf("no ICE signal data")
	}
	if err := json.Unmarshal(f.ICESignal, &signal); err != nil {
		return signal, fmt.Errorf("unmarshal ICE signal: %w", err)
	}
	return signal, nil
}
