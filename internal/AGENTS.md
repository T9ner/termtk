# DOX Framework - Internal Directory Contract

- This directory governs all internal Go packages, domain models, and core logic.
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose and Scope

The `internal/` directory houses the private packages of the TermTalk application. Code in this directory cannot be imported by external projects, enforcing encapsulation.
Key subdirectories:
- [db/](file:///C:/Users/HP/Desktop/termtk/internal/db): Managed by [db/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/db/AGENTS.md). SQLite-based database interactions, profile management, and chat history.
- [network/](file:///C:/Users/HP/Desktop/termtk/internal/network): Managed by [network/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/network/AGENTS.md). UDP-based peer discovery and TCP-based peer synchronization protocols.

## Directory Index

- [db/](file:///C:/Users/HP/Desktop/termtk/internal/db) -> Managed by [db/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/db/AGENTS.md)
- [network/](file:///C:/Users/HP/Desktop/termtk/internal/network) -> Managed by [network/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/network/AGENTS.md)

## Code Quality Standards

1. **Language & Tooling**:
   - Written in Go. Follow standard Go style guidelines and run `go fmt` and `go vet` on edits.
   - Run existing tests with `go test ./...` before finalizing changes.

2. **Concurrency**:
   - Write concurrent-safe code. Protect shared resources (like maps or caches) using mutexes.
