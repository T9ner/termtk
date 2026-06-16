# DOX Framework — Client Directory Contract

- This directory governs the Client Facade that coordinates all subsystems.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `client` provides the `Client` struct — the central Facade/Mediator that encapsulates database, peer discovery, and sync manager behind a single API. The UI layer communicates exclusively through this package.

## Ownership

- [client.go](file:///C:/Users/HP/Desktop/termtk/internal/client/client.go): `Client` struct, `Event` interface, `PeerDiscoveredEvent`, `MessageReceivedEvent`, `SearchResultEvent`, `OnlineListEvent`, `ReadAckEvent`, and all public methods including `SearchUsers(query)`, `GetOnlineUsers()`, `SendReadAck()`, `GetUnreadCount()`, `MarkMessagesRead()`

## Local Contracts

- **Facade Pattern**: `Client` is the only bridge between `ui` and infrastructure (`db`, `network`). Do NOT let `ui` import `db` or `network` directly
- **Event Channel**: `Client.Events()` returns a buffered `<-chan Event` (capacity 100). Callbacks from `network` push events into this channel. If the channel fills, the network goroutine blocks — keep consumers responsive
- **Event Types**: All event types implement the `Event` interface and live in this package. Add new event types here, not in `ui` or `network`

## Work Guidance

- When adding new functionality, expose it as a method on `Client` — do not have the UI call infrastructure directly
- Keep the `Client` struct thin: it should delegate to `db` and `network`, not contain business logic itself

## Verification

```bash
go test ./internal/client/...
```

## Child DOX Index

No children.
