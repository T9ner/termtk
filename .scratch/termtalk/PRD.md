Status: ready-for-agent

# Product Requirement Document (PRD): TermTalk (Go Edition)

## Problem Statement

Users want a terminal-based messaging application to communicate with their friends across different operating systems (Windows and macOS) without being dependent on a centralized server or even a continuous internet connection. Centralized messaging platforms require accounts, internet connectivity, and external servers, which are prone to downtime, tracking, and lack of offline functionality.

## Solution

TermTalk is a peer-to-peer (P2P), offline-first terminal-based messaging application. It operates in two modes:
1. **Network-connected (Local P2P)**: Auto-discovers other TermTalk instances on the local network using UDP broadcast and syncs/sends messages directly via TCP.
2. **Disconnected (Sneakernet / File Sync)**: Stores messages locally in an SQLite database. Allows exporting outbound messages to a sync file (JSON format) and importing incoming sync files from friends (e.g., via USB drive, email, or local file sharing), ensuring communication works even in zero-network environments.

TermTalk uses **Go** and **Bubble Tea** (Charm.sh) to compile into a single native binary for Windows and macOS, providing a beautiful, modern terminal user interface (TUI) with zero runtime dependencies.

## User Stories

1. As a user, I want to start TermTalk in a terminal and see a clean, modern user interface so that I can easily navigate my chats.
2. As a user, I want to set up my username and profile identifier on first launch so that my friends can identify me.
3. As a user, I want to add friends by their unique identifiers (username/UUID) so that I can initiate conversations with them.
4. As a user, I want my messages to be stored locally on my device in an SQLite database so that I can access them when offline.
5. As a user, I want my TermTalk app to automatically discover friends on the same local Wi-Fi network so that I don't need to manually configure IP addresses.
6. As a user, I want to send real-time text messages to online friends on the same network so that I can chat interactively.
7. As a user, I want to write messages to offline friends so that they are queued for delivery when we connect.
8. As a user, I want to export my pending outbound messages to a sync file (JSON) so that I can send them via a USB flash drive or other physical media.
9. As a user, I want to import a sync file received from a friend so that their messages are merged into my local chat database.
10. As a user, I want to see the status of each message (e.g., "Draft", "Sent", "Synced") in the terminal UI so that I know if it has reached my friend.
11. As a developer, I want the project to support Windows and macOS natively by compiling to standalone binaries without CGO or native C library requirements.

## Implementation Decisions

- **Programming Language**: Go (Golang) 1.22+
- **TUI Framework**: `github.com/charmbracelet/bubbletea` for the Elm architecture loop, `github.com/charmbracelet/lipgloss` for styling, and `github.com/charmbracelet/bubbles` for standard input and viewport components.
- **Database**: SQLite using a pure-Go driver (`modernc.org/sqlite`) to avoid CGO requirements, ensuring seamless cross-compilation.
- **P2P Discovery**: UDP Broadcast on port 55555. Active clients broadcast their presence periodically.
- **Message Transmission**: TCP connections on port 55556 to sync history and transmit messages.
- **Sync File Format**: JSON-based schema storing a list of messages with unique hashes, timestamps, sender/recipient UUIDs, and a sync signature.
- **Security / Message Integrity**: Every message contains a hash of its contents, sender, recipient, and timestamp to ensure integrity and deduplication.

## Testing Decisions

- **Testing Seams**: We will decouple the networking engine (UDP/TCP sockets), the database operations, and the Bubble Tea TUI model.
- **Unit Tests**: Test database queries, message hash generation, sync export/import serialization, and deduplication logic using standard Go tests (`go test`).
- **Mocking**: Mock net.Conn and net.PacketConn interfaces to verify peer discovery and direct sync exchange flows.
- **Manual Verification**: Run multiple instances locally on different ports to simulate multi-node behavior.

## Out of Scope

- Centralized server hosting and cloud sync.
- Cryptographic public-key end-to-end encryption (out of scope for MVP to keep implementation lightweight, but message signatures/hashes are implemented for deduplication).
- Multimedia attachments (images, audio, video).

## Further Notes

TermTalk should be easily compileable via `go build` and runnable locally. It stores its SQLite database in a user-standard config folder (e.g., `AppData/Roaming/termtalk` on Windows or `~/.config/termtalk` on macOS).
