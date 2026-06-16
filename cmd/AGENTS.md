# DOX Framework — Command Entry Points Contract

- This directory governs the application entry points (binaries).
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose

The `cmd/` directory contains the `main` packages for TermTalk's two binaries:
- **termtalk**: The client application (TUI + P2P messaging)
- **termtalk-relay**: The decentralized TCP signaling and routing relay server

## Ownership

- [termtalk/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk/main.go): Client entry point — wires SQLite DB, UDP discovery, TCP sync, and launches the Bubble Tea TUI
- [termtalk-relay/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/main.go): Relay server — `RelayServer` struct manages client registrations, message routing, store-and-forward for offline recipients, user registry, search, who_online presence, and heartbeat keepalive
- [termtalk-relay/relay_store_test.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/relay_store_test.go): Tests for store-and-forward, flush-on-reconnect, delivery receipts, search, online status, and empty-query behavior (8 tests)

## Local Contracts

- **Wiring Only**: `main.go` files should only contain dependency initialization and wiring. Business logic belongs in `internal/`
- **Shared Protocol Types**: Both binaries use `RelayFrame` from [internal/protocol](file:///C:/Users/HP/Desktop/termtk/internal/protocol/frames.go). Do NOT redefine protocol types locally

## Work Guidance

- Cross-compilation is enabled via `CGO_ENABLED=0` — do not add CGO dependencies
- GoReleaser handles release builds — see [.goreleaser.yaml](file:///C:/Users/HP/Desktop/termtk/.goreleaser.yaml)

## Verification

```bash
go build ./cmd/termtalk
go build ./cmd/termtalk-relay
```

## Child DOX Index

No children.
