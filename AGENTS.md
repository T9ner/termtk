# DOX Framework — TermTalk Root Contract

- DOX is a self-maintaining AGENTS.md hierarchy installed in this project
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read this root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant (`go run scripts/validate.go`)
6. Report any docs intentionally left unchanged and why

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history

---

## Project Overview

**TermTalk** is an offline-first, decentralized terminal-based messaging application written in Go.

### Workspace Index

- [cmd/termtalk/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk/main.go): Application entry point — initializes SQLite database, UDP discovery, TCP synchronization managers, and runs the TUI.
- [cmd/termtalk-relay/main.go](file:///C:/Users/HP/Desktop/termtk/cmd/termtalk-relay/main.go): Decentralized TCP signaling and routing relay server.
- [internal/](file:///C:/Users/HP/Desktop/termtk/internal): All private Go packages. Managed by [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md).
- [docs/](file:///C:/Users/HP/Desktop/termtk/docs): Project documentation, ADRs, and CE lessons. Managed by [docs/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/docs/AGENTS.md).
- [.agents/skills/](file:///C:/Users/HP/Desktop/termtk/.agents/skills): Agent skills for this workspace. Managed by [.agents/skills/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/.agents/skills/AGENTS.md).
- [scripts/validate.go](file:///C:/Users/HP/Desktop/termtk/scripts/validate.go): Verification gate — runs fmt → vet → test → cross-compile.
- [LICENSE](file:///C:/Users/HP/Desktop/termtk/LICENSE): MIT License (2026, TermTalk Contributors).
- [Dockerfile](file:///C:/Users/HP/Desktop/termtk/Dockerfile): Multi-stage Docker build for the relay server (Fly.io deployment).
- [fly.toml](file:///C:/Users/HP/Desktop/termtk/fly.toml): Fly.io deployment config — London (lhr) primary region, TCP passthrough.
- [.dockerignore](file:///C:/Users/HP/Desktop/termtk/.dockerignore): Docker build context exclusions.

### Key Reference Documents

- [CONTEXT.md](file:///C:/Users/HP/Desktop/termtk/CONTEXT.md): Ubiquitous language definitions (Peer, Outbox, Discovery, TCP Sync).
- [CLAUDE.md](file:///C:/Users/HP/Desktop/termtk/CLAUDE.md): Quick command reference.
- [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md): Compound Engineering lessons log and regression-prevention memory.
- [.scratch/termtalk/PRD.md](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/PRD.md): Product Requirement Document.
- [.scratch/termtalk/issues/](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/): Local issue tracker with completed implementation tasks.

## Compound Engineering Instructions

1. **Read-First Directive:** When starting a session or implementing a task, you MUST read [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md) to ensure no optimization regressions are introduced.
2. **Log-Upon-Fix:** If you resolve a bug, complete a major refactoring, or apply a performance speedup:
   - Run verification builds and tests.
   - Document the problem, root cause, code change, and prevention rules inside [docs/ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md).
3. **Verification Gate:** Before proposing any code changes to the user or closing an issue, the agent MUST run the validation script using `go run scripts/validate.go` and verify it passes (exit code 0).

## User Preferences

- Go with CGO-free SQLite (`github.com/ncruces/go-sqlite3`) for cross-compilation
- Bubble Tea / Lipgloss for TUI
- Content-addressed message IDs (SHA-256)
- Offline-first, decentralized architecture
- Compound Engineering lessons documented in `docs/ce_lessons.md`

## Child DOX Index

- [cmd/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/cmd/AGENTS.md) — Entry points: client binary and relay server
- [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md) — All private Go packages (db, network, client, protocol, ui)
  - [internal/db/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/db/AGENTS.md) — SQLite persistence, models, sneakernet sync
  - [internal/network/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/network/AGENTS.md) — UDP discovery, TCP sync, relay client
  - [internal/client/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/client/AGENTS.md) — Client Facade coordinating db, network, and events
  - [internal/protocol/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/protocol/AGENTS.md) — Shared wire-format types (RelayFrame)
  - [internal/ui/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/ui/AGENTS.md) — Bubble Tea TUI: model, update, view
- [docs/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/docs/AGENTS.md) — ADRs, CE lessons, domain vocabulary, triage
- [.agents/skills/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/.agents/skills/AGENTS.md) — Installed and custom agent skills
