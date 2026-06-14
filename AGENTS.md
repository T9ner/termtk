# AGENTS.md - TermTalk Workspace Index

This workspace contains the source code, tests, designs, and tasks for **TermTalk**—an offline-first, decentralized terminal-based messaging application written in Go.

## Workspace Overview

*   [cmd/termtalk/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk/main.go): Application entry point, initializing SQLite database, UDP discovery, TCP synchronization managers, and running the TUI.
*   [cmd/termtalk-relay/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/main.go): Decentralized TCP signaling and routing relay server.
*   [internal/db/](file:///C:/Users/HP/Desktop/termtk/internal/db): Data persistence layer using pure Go SQLite (`github.com/ncruces/go-sqlite3`) to allow CGO-free cross-compiling for macOS and Windows.
    *   [models.go](file:///C:/Users/HP/Desktop/termtk/internal/db/models.go): Profile, Contact, and Message structs.
    *   [db.go](file:///C:/Users/HP/Desktop/termtk/internal/db/db.go): SQLite schema migrations and operations.
    *   [sneakernet.go](file:///C:/Users/HP/Desktop/termtk/internal/db/sneakernet.go): JSON-based sync file export/import for network-free file sharing.
*   [internal/network/](file:///C:/Users/HP/Desktop/termtk/internal/network): P2P networking protocols.
    *   [discovery.go](file:///C:/Users/HP/Desktop/termtk/internal/network/discovery.go): UDP broadcast daemon for auto-detecting peers on local Wi-Fi.
    *   [sync.go](file:///C:/Users/HP/Desktop/termtk/internal/network/sync.go): TCP sync manager that negotiates message history and sends real-time messages.
    *   [relay_test.go](file:///C:/Users/HP/Desktop/termtk/internal/network/relay_test.go): Integration tests for message relay routing.
*   [internal/ui/](file:///C:/Users/HP/Desktop/termtk/internal/ui): Bubble Tea TUI implementation.
    *   [types.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/types.go): Bubble Tea message types.
    *   [model.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/model.go): State models and text inputs.
    *   [update.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/update.go): Elm Architecture update loop & shortcut commands.
    *   [view.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/view.go): Lipgloss layouts, colors, and dashboard renderers.

## Project Resources

*   [CONTEXT.md](file:///C:/Users/HP/Desktop/termtk/CONTEXT.md): Ubiquitous language definitions (Peer, Outbox, Discovery, TCP Sync).
*   [CLAUDE.md](file:///C:/Users/HP/Desktop/termtk/CLAUDE.md): Quick command reference.
*   [PRD.md](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/PRD.md): Product Requirement Document.
*   [issues/](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/): Local issue tracker containing the completed implementation tasks.
*   [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md): Compound Engineering lessons log and regression-prevention memory.

## Compound Engineering Instructions

1. **Read-First Directive:** When starting a session or implementing a task, you MUST read [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md) to ensure no optimization regressions are introduced.
2. **Log-Upon-Fix:** If you resolve a bug, complete a major refactoring, or apply a performance speedup:
   - Run verification builds and tests.
   - Document the problem, root cause, code change, and prevention rules inside [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md).
3. **Verification Gate:** Before proposing any code changes to the user or closing an issue, the agent MUST run the validation script using `go run scripts/validate.go` and verify it passes (exit code 0).

