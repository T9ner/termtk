# DOX Framework — Internal Directory Contract

- This directory governs all private Go packages, domain models, and core logic.
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose

The `internal/` directory houses the private packages of the TermTalk application. Code in this directory cannot be imported by external projects, enforcing encapsulation.

## Ownership

All Go packages under `internal/` are owned by this contract. Each subdirectory with its own AGENTS.md owns its local rules.

## Local Contracts

- **Language & Tooling**: Written in Go. Run `go fmt` and `go vet` on all edits.
- **Testing**: Run `go test ./...` before finalizing changes.
- **Concurrency**: Protect shared resources (maps, caches, encoders) using `sync.Mutex` or `sync.RWMutex`. Use `sync.Once` for lifecycle channel closes. See [CE-002](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md) and [CE-003](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md).

## Work Guidance

- Follow the 4-layer architecture: `cmd/` (entry) → `ui` (presentation) → `client` (facade/service) → `db` + `network` (infrastructure)
- The UI must never import `db` or `network` directly — always go through `client`
- Align naming with [CONTEXT.md](file:///C:/Users/HP/Desktop/termtk/CONTEXT.md) ubiquitous language

## Verification

```bash
go test ./internal/...
go vet ./internal/...
```

## Child DOX Index

- [db/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/db/AGENTS.md) — SQLite persistence, models, sneakernet sync
- [network/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/network/AGENTS.md) — UDP discovery, TCP sync, relay client
- [client/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/client/AGENTS.md) — Client Facade coordinating db, network, and events
- [protocol/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/protocol/AGENTS.md) — Shared wire-format types (RelayFrame)
- [ui/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/ui/AGENTS.md) — Bubble Tea TUI: model, update, view
