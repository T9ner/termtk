# DOX Framework - Network Directory Contract

- This directory governs all networking, peer discovery protocols, and synchronization mechanisms.
- Parent: [Internal AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose and Scope

This package (`network`) handles all network-related communication for TermTalk, enabling real-time discovery and message exchange between local network peers.
Key responsibilities:
- **Peer Discovery**: UDP broadcast daemon that broadcasts and listens for active TermTalk peers on `DiscoveryPort` (55555) using [discovery.go](file:///C:/Users/HP/Desktop/termtk/internal/network/discovery.go).
- **Contact Integration**: Automatic registration of discovered peers in the local database.
- **Sync Protocol (Future)**: Establishing TCP-based connections to synchronize message logs.

## Guidelines for Network Work

1. **Graceful Shutdown**:
   - Ensure network listeners (UDP/TCP connections) are gracefully closed during stopping sequences to prevent port binding conflicts.

2. **Network Robustness**:
   - Broadcast packets are sent over UDP. Handle network timeouts, packet losses, and packet formatting issues gracefully.
