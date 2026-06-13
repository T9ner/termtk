Status: completed

# Issue 3: Direct Messaging & TCP Sync Protocol

## What to build

Implement a reliable P2P synchronization and direct messaging protocol over TCP. When a peer is discovered online, establish a TCP connection, perform a handshake, exchange message history hashes, transfer missing messages, and maintain the socket for real-time messaging.

## Acceptance criteria

- [ ] TCP Server listening on a configurable port (default 55556).
- [ ] Direct TCP Client connection manager that connects to discovered peers.
- [ ] Handshake protocol exchanging peer identity details.
- [ ] Message history synchronization exchange:
  - Peers send lists of message IDs (hashes) they have in their database.
  - Peers request missing message IDs.
  - Senders transmit requested messages.
- [ ] Real-time instant messaging: when typing a message to an active peer, send it over the active TCP connection and mark it as `synced` in the local DB.
- [ ] Graceful connection teardown and reconnection logic if network is interrupted.
- [ ] Covered by unit tests using mock TCP sockets.

## Blocked by

- [Issue 1: Database Setup and Local Message Store](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/01-database-store.md)
- [Issue 2: P2P Network Discovery over UDP](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/02-peer-discovery.md)
