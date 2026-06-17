# DOX Framework — Command Entry Points Contract

- This directory governs the application entry points (binaries).
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose

The `cmd/` directory contains the `main` packages for TermTalk's two binaries:
- **termtalk**: The client application (TUI + P2P messaging)
- **termtalk-relay**: The decentralized TCP signaling and routing relay server

## Ownership

- [termtalk/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk/main.go): Client entry point — wires SQLite DB, UDP discovery, TCP sync, and launches the Bubble Tea TUI
- [termtalk-relay/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/main.go): Relay server — `RelayServer` struct manages client registrations, message routing, store-and-forward for offline recipients, user registry, search, who_online presence, heartbeat keepalive, graceful shutdown, and periodic health check logging (`ConnectedCount`, `RegisteredCount`, `StoredMessageCount`)
- [termtalk-relay/store.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/store.go): `RelayStore` — SQLite persistence layer for the relay server (user registry + stored messages). Uses `ncruces/go-sqlite3` (CGO-free). DB path defaults to `/data/relay.db` (Fly.io volume) or `./relay.db` (local dev). Schema: `users` table (uuid, username, public_key, last_seen) and `stored_messages` table (sender, recipient, JSON payload, message_id, created_at)
- [termtalk-relay/relay_store_test.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/relay_store_test.go): Tests for store-and-forward, flush-on-reconnect, delivery receipts, search, online status, and empty-query behavior (8 tests). Tests pass `nil` store to use pure in-memory mode

## Deployment Infrastructure (Root-Level)

- [Dockerfile](file:///C:/Users/HP/Desktop/termtk/Dockerfile): Multi-stage build for the relay server (Go 1.24 Alpine → Alpine 3.20 runtime). CGO_ENABLED=0 for cross-compilation. `/data` volume for SQLite persistence
- [fly.toml](file:///C:/Users/HP/Desktop/termtk/fly.toml): Fly.io deployment config — raw TCP passthrough on port 55558, London (lhr) primary region, TCP health check, `termtalk_data` volume mounted at `/data`
- [.dockerignore](file:///C:/Users/HP/Desktop/termtk/.dockerignore): Excludes SQLite databases, scratch/agent directories, and Windows executables from build context

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
