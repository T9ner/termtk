# ADR-003: Desktop App Architecture (Wails + xterm.js Terminal Shell)

**Date:** 2026-06-17  
**Status:** Accepted  
**Deciders:** User + Agent

## Context

Nod is a terminal-based messaging app. Distribution to non-technical users is painful — they must download a binary, open a terminal, navigate to it, and run it. Users on old versions must manually download updates from GitHub. There's no way to receive OS notifications or minimize to system tray.

## Decision

Build a **Wails desktop wrapper** (`cmd/Nod-desktop/`) that:
- Opens a native Windows window with **xterm.js** rendering the existing Bubble Tea TUI
- Communicates via **ConPTY** — spawns `Nod.exe` as a child process with a pseudo-terminal
- Ships as a **side-by-side pair** (`Nod.exe` + `Nod-desktop.exe`) via an **Inno Setup installer**
- Targets **Windows only** for v1.0 (macOS in v1.1)
- Stores data in **`%APPDATA%\Nod\`** (shared between CLI and desktop)
- Keeps **SQLite** as the database (unchanged)

### v1.0 Features
- System tray icon (minimize-to-tray, connection status)
- OS toast notifications on incoming messages
- Auto-update via GitHub Releases (go-selfupdate)
- Single instance lock
- Custom app icon and branding

### Migration Path
1. v0.4.1: Move data directory to `%APPDATA%\Nod\`
2. v0.5.0: goreleaser + auto-update version checking
3. v1.0.0: Wails desktop wrapper + Inno Setup installer

## Consequences

- **Zero changes to TUI code** — Bubble Tea runs unmodified inside the PTY
- **Two distribution paths** — CLI for power users, desktop app for everyone else
- **npm added to build toolchain** — Wails frontend needs xterm.js dependency
- **WebView2 required** — preinstalled on Windows 10 21H2+ and all Windows 11
- Full architecture doc: `.scratch/Nod/desktop_app_architecture.md`
