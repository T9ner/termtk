# TermTalk 💬

A terminal-based instant messenger. Find people, add contacts, and chat — all from your terminal.

**No servers to set up. No UUIDs to exchange. Just run it.**

---

## Getting Started

### 1. Download

Grab the latest binary for your OS from [GitHub Releases](https://github.com/T9ner/termtk/releases):

| OS | Binary |
|----|--------|
| Windows | `termtalk_windows_amd64.exe` |
| macOS | `termtalk_darwin_amd64` |
| Linux | `termtalk_linux_amd64` |

### 2. Run

```bash
# macOS / Linux
chmod +x termtalk_*
./termtalk_linux_amd64

# Windows
termtalk_windows_amd64.exe
```

That's it. No flags, no config files, no server setup.

### 3. Register

Pick a username. Your unique ID is generated automatically.

```
  Welcome to TermTalk

> Enter your username: tunde
```

You're now connected to the TermTalk relay and visible to other users.

### 4. Find People

Press **`Ctrl+F`** to search for other TermTalk users:

```
  Find Users on Relay

Search: > chi
  [ON]  chidi
  [OFF] chioma

↑↓: Navigate | Enter: Add Contact | Esc: Cancel
```

Select a user and press **Enter** to add them as a contact.

### 5. Chat

Select a contact in the sidebar, press **Tab** to switch to the chat pane, type your message, and hit **Enter**.

```
  @tunde  |  Ctrl+P: Profile  |  Ctrl+F: Find Users

┌─────────────┐  ┌──────────────────────────────┐
│ Contacts    │  │ Chatting with chidi (online)  │
│             │  │                               │
│ > chidi (2) │  │ chidi: hey!                   │
│   alice     │  │ you: what's good?         [✓] │
│             │  │ chidi: nm, you?               │
│             │  │ you: building termtalk    [✓✓] │
│             │  │                               │
│             │  │ > type a message...           │
└─────────────┘  └──────────────────────────────┘
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+F` | Find users on the relay |
| `Ctrl+P` | View your profile and shareable handle |
| `Tab` | Switch between sidebar and chat |
| `↑` `↓` | Navigate contacts or search results |
| `Enter` | Send message / Select contact |
| `Ctrl+E` | Export sync file (sneakernet) |
| `Ctrl+I` | Import sync file |
| `Ctrl+C` | Quit |

---

## Message Status

| Icon | Meaning |
|------|---------|
| `[Queued]` | Saved locally, waiting for relay connection |
| `[✓]` | Delivered to recipient |
| `[✓✓]` | Read by recipient |

---

## How It Works

TermTalk connects to a cloud relay at `termtalk-relay.fly.dev` for user discovery and message delivery. On local networks, it also uses UDP broadcast for direct peer-to-peer connections.

```
┌──────────┐          ┌─────────────────────┐          ┌──────────┐
│  Tunde   │◄── TCP ──┤  Cloud Relay (Fly)  ├── TCP ──►│  Chidi   │
│ (Lagos)  │          │  Search, Store &    │          │ (Campus) │
│          │          │  Forward, Presence  │          │          │
└──────────┘          └─────────────────────┘          └──────────┘
      ▲                                                       ▲
      └──── UDP (auto-discover if same LAN) ──────────────────┘
```

**Key features:**
- **Store & Forward**: Messages are stored on the relay when you're offline and delivered when you reconnect
- **Read Receipts**: See when your messages are delivered and read
- **Unread Badges**: Contact list shows unread message counts
- **Live Presence**: See who's online in real-time
- **Offline-First**: All messages stored locally in SQLite, works without internet for LAN chat

---

## Installation Alternatives

### Homebrew (macOS / Linux)

```bash
brew tap T9ner/homebrew-tap
brew install termtalk
```

### WinGet (Windows)

```cmd
winget install T9ner.TermTalk
```

### Build from Source

```bash
git clone https://github.com/T9ner/termtk.git
cd termtk
go build -o termtalk ./cmd/termtalk
./termtalk
```

---

## Self-Hosting the Relay

You can run your own relay server:

```bash
# Build and run locally
go run ./cmd/termtalk-relay --port 55558

# Connect clients to your relay
./termtalk --relay your-server.com:55558
```

Or deploy to Fly.io:

```bash
fly launch --name my-relay --region lhr
fly deploy
```

---

## Development

```bash
# Run tests
go test ./...

# Run validation gate (fmt + vet + test + cross-compile)
go run scripts/validate.go

# Format code
go fmt ./...
```

---

## Architecture

| Component | Role |
|-----------|------|
| `cmd/termtalk` | Client binary — TUI, networking, local DB |
| `cmd/termtalk-relay` | Relay server — user registry, store-and-forward, search, presence |
| `internal/ui` | Bubble Tea terminal UI |
| `internal/client` | Client facade — orchestrates DB, networking, events |
| `internal/network` | TCP sync, UDP discovery, relay connection |
| `internal/db` | SQLite storage (CGO-free, pure Go) |
| `internal/protocol` | Wire protocol frame types |

---

**Built with** [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lip Gloss](https://github.com/charmbracelet/lipgloss) · [ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) · Deployed on [Fly.io](https://fly.io)
