# DOX Framework — Network Directory Contract

- This directory governs all P2P networking, peer discovery, and message synchronization.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `network` handles all network communication for TermTalk — real-time peer discovery via UDP broadcast and message synchronization via TCP (both local P2P and relay-routed).

## Ownership

- [discovery.go](file:///C:/Users/HP/Desktop/termtk/internal/network/discovery.go): UDP broadcast daemon for auto-detecting peers on local Wi-Fi (`DiscoveryPort` 55555)
- [sync.go](file:///C:/Users/HP/Desktop/termtk/internal/network/sync.go): TCP sync manager — peer handshakes, message history negotiation, relay client lifecycle, keepalive heartbeat, search/who_online requests, store-and-forward ack handling (stored/delivered), and outbox drain on reconnect

## Local Contracts

- **Encoder Thread Safety**: Do NOT perform concurrent writes on `json.Encoder` instances without per-connection mutex locks. See [CE-002](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md)
- **Channel Close Safety**: Use `sync.Once` for closing coordination channels like `pingStop` and `stopChan`. Both `SyncManager.Stop()` and `PeerDiscovery.Stop()` use `sync.Once`. Do NOT close channels in multiple goroutines. See [CE-003](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md)
- **Context Propagation**: `Start()` and `ConnectToPeer()` accept `context.Context`. Use context deadlines instead of manual `SetDeadline` calls. Pass context to child operations for structured cancellation
- **Connection Cleanup**: Always verify mapping integrity (`clients[uuid] == client`) before deleting clients from maps inside deferred disconnect handlers
- **Graceful Shutdown**: Network listeners (UDP/TCP) must be gracefully closed during stop sequences to prevent port binding conflicts

## Work Guidance

- `sync.go` is the largest file (739 lines). Future refactoring should split into local sync and relay sync
- Broadcast packets are UDP — handle timeouts, packet losses, and formatting issues gracefully
- The `SyncManager` exposes callbacks (`OnMsgRecv`, `OnSearchResult`, `OnOnlineList`) — these run on network goroutines, so keep them non-blocking
- `SendSearchRequest(query)` and `SendWhoOnline()` encode frames directly to the relay encoder — call only when relay is online
- `drainOutbox()` runs automatically on relay connect — re-sends all locally queued messages

## Verification

```bash
go test ./internal/network/...
```

## Child DOX Index

No children.
