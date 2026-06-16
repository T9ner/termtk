package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"termtalk/internal/db"
)

const (
	DiscoveryPort     = 55555
	DiscoveryPayload  = "termtalk:discovery:%s:%s:%d" // termtalk:discovery:UUID:USERNAME:TCP_PORT
	BroadcastInterval = 5 * time.Second
)

// PeerDiscovery handles local network peer detection via UDP broadcast.
type PeerDiscovery struct {
	localUUID   string
	username    string
	tcpPort     int
	database    *db.Database
	activePeers map[string]*db.Contact
	mu          sync.Mutex
	stopChan    chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	OnPeerFound func(contact *db.Contact)
}

// NewPeerDiscovery creates a new PeerDiscovery manager.
func NewPeerDiscovery(localUUID, username string, tcpPort int, database *db.Database) *PeerDiscovery {
	return &PeerDiscovery{
		localUUID:   localUUID,
		username:    username,
		tcpPort:     tcpPort,
		database:    database,
		activePeers: make(map[string]*db.Contact),
		stopChan:    make(chan struct{}),
	}
}

// UpdateCredentials updates the profile credentials thread-safely after registration.
func (p *PeerDiscovery) UpdateCredentials(uuid, username string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.localUUID = uuid
	p.username = username
}

// getCredentials safely reads credentials under lock.
func (p *PeerDiscovery) getCredentials() (string, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.localUUID, p.username
}

// Start spawns the listener and announcer background loops.
func (p *PeerDiscovery) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Start listener
	listenerConn, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", DiscoveryPort))
	if err != nil {
		return fmt.Errorf("failed to bind UDP discovery port: %w", err)
	}

	p.wg.Add(2)
	go p.listenLoop(listenerConn)
	go p.announceLoop()

	return nil
}

// Stop stops the discovery engine.
func (p *PeerDiscovery) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
	})
	p.wg.Wait()
}

// listenLoop processes incoming UDP discovery packets.
func (p *PeerDiscovery) listenLoop(conn net.PacketConn) {
	defer p.wg.Done()
	defer conn.Close()

	buf := make([]byte, 1024)

	// Set read deadline so the socket check isn't indefinitely blocking and can stop gracefully
	go func() {
		<-p.stopChan
		conn.Close() // Force-close the connection to break read block
	}()

	for {
		select {
		case <-p.stopChan:
			return
		default:
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				// Handle socket closed error gracefully on stop
				select {
				case <-p.stopChan:
					return
				default:
					time.Sleep(1 * time.Second) // Wait and retry on temporary error
					continue
				}
			}

			payload := string(buf[:n])
			if !strings.HasPrefix(payload, "termtalk:discovery:") {
				continue
			}

			parts := strings.Split(payload, ":")
			if len(parts) < 5 {
				continue
			}

			peerUUID := parts[2]
			peerPortStr := parts[len(parts)-1]
			// Join remaining segments as username in case it contains colons
			peerUsername := strings.Join(parts[3:len(parts)-1], ":")

			// Ignore self-announcements
			localUUID, _ := p.getCredentials()
			if peerUUID == localUUID {
				continue
			}

			peerPort, err := strconv.Atoi(peerPortStr)
			if err != nil {
				continue
			}

			udpAddr, ok := addr.(*net.UDPAddr)
			if !ok {
				continue
			}

			peerIP := udpAddr.IP.String()

			contact := &db.Contact{
				UUID:     peerUUID,
				Username: peerUsername,
				IP:       peerIP,
				Port:     peerPort,
				LastSeen: time.Now(),
			}

			// Save to database
			if p.database != nil {
				if err := p.database.UpsertContact(contact); err != nil {
					log.Printf("discovery: failed to upsert contact %s: %v", peerUUID, err)
				}
			}

			// Update active peers list
			p.mu.Lock()
			p.activePeers[peerUUID] = contact
			p.mu.Unlock()

			if p.OnPeerFound != nil {
				p.OnPeerFound(contact)
			}
		}
	}
}

// announceLoop periodically sends out UDP broadcast packets to notify other peers.
func (p *PeerDiscovery) announceLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			localUUID, username := p.getCredentials()
			payload := fmt.Sprintf(DiscoveryPayload, localUUID, username, p.tcpPort)
			p.broadcast(payload)
		}
	}
}

// broadcast broadcasts a payload to the local subnet.
func (p *PeerDiscovery) broadcast(payload string) {
	conn, err := net.Dial("udp4", fmt.Sprintf("255.255.255.255:%d", DiscoveryPort))
	if err != nil {
		return // Ignore error, retry on next tick
	}
	defer conn.Close()

	_, _ = conn.Write([]byte(payload))
}

// GetActivePeers returns a slice of all active peers detected during this session.
func (p *PeerDiscovery) GetActivePeers() []*db.Contact {
	p.mu.Lock()
	defer p.mu.Unlock()

	var peers []*db.Contact
	for _, peer := range p.activePeers {
		peers = append(peers, peer)
	}
	return peers
}
