# DOX Framework — UI Directory Contract

- This directory governs the Bubble Tea TUI layer.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `ui` implements the terminal user interface using [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm Architecture) and [Lipgloss](https://github.com/charmbracelet/lipgloss) for styling.

## Ownership

- [model.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/model.go): `Model` struct, `NewModel()`, `Init()` — state initialization, text inputs, `FocusMode` type and `AppState` constants
- [update.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/update.go): `Update()` — Elm Architecture message loop, keyboard handling, shortcut commands, focus-aware input routing
- [view.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/view.go): `View()` — Lipgloss layouts, color palette, dashboard, profile, chat, and empty-state renderers

## Local Contracts

- **No Direct Infrastructure Imports**: This package must NOT import `internal/db` or `internal/network`. All data access goes through `internal/client.Client`
- **Elm Architecture**: `Init() → Update() → View()` — `Update()` is a pure function (receives `tea.Msg`, returns `Model` + `tea.Cmd`). Keep it testable
- **Event Subscription**: `ListenForEvents()` is a `tea.Cmd` that blocks on `Client.Events()` channel. It bridges async network events into the Bubble Tea message loop

## Focus Mode System

The dashboard uses a `FocusMode` enum (`FocusSidebar` / `FocusChat`) to control which pane owns keyboard input:

- **Tab key** toggles focus between sidebar and chat panes
- **FocusSidebar**: Up/Down navigate contacts, Enter opens chat with selected contact (switches to FocusChat)
- **FocusChat**: Up/Down scroll the viewport, Enter sends a message, character keys route to the message input
- Visual indicator: active pane gets accent-colored border (sidebar) or underlined header (chat)
- Note: `tea.KeyTab` and `tea.KeyCtrlI` are the same key code (ASCII 9), so Import uses **Ctrl+O** instead of Ctrl+I

## App States

| State | Purpose | Escape Route |
|-------|---------|--------------|
| `StateRegister` | First-boot username prompt | Ctrl+C |
| `StateDashboard` | Main split-pane view | Esc / Ctrl+Q quits |
| `StateProfile` | Read-only profile card with share ID | Esc → Dashboard |
| `StateExport` | Export sync file path prompt | Esc → Dashboard |
| `StateImport` | Import sync file path prompt | Esc → Dashboard |
| `StateAddContact` | Manual contact entry | Esc → Dashboard |

## Context-Aware Footer

The footer changes shortcut hints based on the current state and focus:

- **FocusSidebar**: `↑↓: Navigate | Enter: Open Chat | Tab: Switch to Chat | Ctrl+N: Add Peer | Ctrl+P: Profile | Ctrl+Q: Quit`
- **FocusChat**: `↑↓: Scroll | Enter: Send | Tab: Switch to Contacts | Ctrl+E: Export | Ctrl+O: Import | Ctrl+Q: Quit`
- **Other states**: `Enter: Confirm | Esc: Cancel` (or state-specific hints)

## Work Guidance

- `Update()` uses value receivers per Bubble Tea convention — the entire Model is copied on every update. Be mindful of allocation pressure with large slices
- Viewport height calculations must subtract headers and borders dynamically. See [CE-002](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md) for past layout bugs
- [update_test.go](file:///C:/Users/HP/Desktop/termtk/internal/ui/update_test.go) covers `Update()` and `View()` — key handling, state transitions, event processing, window resize, focus toggling, and render-without-panic for all states

## Verification

```bash
go test ./internal/ui/...
```

## Child DOX Index

No children.
