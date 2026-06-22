package network

import (
	"context"
	"testing"
)

func TestNewPeerDiscovery_ValidInstance(t *testing.T) {
	pd := NewPeerDiscovery("test-uuid", "testuser", 9999, nil)
	if pd == nil {
		t.Fatal("expected non-nil PeerDiscovery")
	}
	if pd.localUUID != "test-uuid" {
		t.Errorf("expected localUUID 'test-uuid', got '%s'", pd.localUUID)
	}
	if pd.username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", pd.username)
	}
	if pd.tcpPort != 9999 {
		t.Errorf("expected tcpPort 9999, got %d", pd.tcpPort)
	}
	if pd.activePeers == nil {
		t.Error("expected activePeers map to be initialized")
	}
	if pd.stopChan == nil {
		t.Error("expected stopChan to be initialized")
	}
}

func TestPeerDiscovery_UpdateCredentials(t *testing.T) {
	pd := NewPeerDiscovery("old-uuid", "olduser", 8888, nil)

	pd.UpdateCredentials("new-uuid", "newuser")

	uuid, username := pd.getCredentials()
	if uuid != "new-uuid" {
		t.Errorf("expected uuid 'new-uuid', got '%s'", uuid)
	}
	if username != "newuser" {
		t.Errorf("expected username 'newuser', got '%s'", username)
	}
}

func TestPeerDiscovery_GetActivePeers_Empty(t *testing.T) {
	pd := NewPeerDiscovery("test-uuid", "testuser", 9999, nil)

	peers := pd.GetActivePeers()
	if len(peers) != 0 {
		t.Errorf("expected 0 active peers, got %d", len(peers))
	}
}

func TestPeerDiscovery_StopWithoutStart(t *testing.T) {
	// Verify that calling Stop without Start doesn't panic
	pd := NewPeerDiscovery("test-uuid", "testuser", 9999, nil)
	pd.Stop() // Should not panic
}

func TestPeerDiscovery_StartStop_Lifecycle(t *testing.T) {
	// Start the discovery engine on the default DiscoveryPort.
	// This may fail if the port is already in use (e.g. another test
	// or a running Nod instance), so we skip rather than fail.
	pd := NewPeerDiscovery("lifecycle-uuid", "lifecycle", 7777, nil)

	err := pd.Start(context.Background())
	if err != nil {
		t.Skipf("skipping lifecycle test: UDP port %d unavailable: %v", DiscoveryPort, err)
	}

	// Verify Stop completes without hanging or panicking
	pd.Stop()
}

func TestPeerDiscovery_DoubleStop(t *testing.T) {
	// Verify that calling Stop twice doesn't panic (sync.Once guards the channel close)
	pd := NewPeerDiscovery("double-uuid", "double", 7778, nil)

	err := pd.Start(context.Background())
	if err != nil {
		t.Skipf("skipping double-stop test: UDP port %d unavailable: %v", DiscoveryPort, err)
	}

	pd.Stop()
	pd.Stop() // Second Stop should be safe
}
