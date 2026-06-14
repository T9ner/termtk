# TermTalk 💬

An offline-first, decentralized, peer-to-peer terminal-based messaging application written in Go.

TermTalk is designed for local network instant messaging and asynchronous message synchronization. It runs entirely without a centralized server, utilizing peer discovery and peer-to-peer synchronization protocols to keep your conversations up to date.

---

## Features

- **Decentralized P2P Messaging**: Communicate directly with other active peers without intermediaries.
- **Automated Peer Discovery**: Auto-detect online contacts on the same local Wi-Fi network using UDP broadcast.
- **State Synchronization (TCP Sync)**: Dynamically exchange message history logs, request missing messages, and maintain open connections for real-time chat.
- **Sneakernet Sync**: Export and import JSON sync files to share messages across disconnected nodes via USB drives or physical media.
- **Bubble Tea Terminal UI**: A sleek, keyboard-driven terminal dashboard built using the Bubble Tea framework.
- **CGO-Free Persistence**: Leverages a pure Go SQLite engine for hassle-free cross-compiling on Windows, macOS, and Linux.

---

## Architecture & Domain Model

- **Peer**: An active TermTalk instance, identified by a unique UUID and username.
- **Contact**: A registered peer saved in the local database.
- **Discovery Daemon**: Broadcasts node credentials and listens on UDP port `55555` to automatically construct the contact registry.
- **TCP Sync Protocol**: Negotiates history by exchanging message hashes (SHA-256) upon connection, requests missing messages, and streams instant messaging.
- **Outbox Queue**: Queues unsent messages locally until the recipient peer comes online.

---

## Installation & Distribution

Instead of executing raw binaries, TermTalk supports distribution through native package managers.

### 🍺 macOS & Linux (Homebrew)

Once released, you can install TermTalk via a Homebrew tap:

```bash
# Tap the repository
brew tap T9ner/homebrew-tap

# Install TermTalk
brew install termtalk
```

**Build from Source locally**:
You can compile and install TermTalk directly from the local formula file:
```bash
brew install --build-from-source ./packaging/homebrew/termtalk.rb
```

---

### 📦 Windows (WinGet)

Once published to the WinGet Community Repository, you can install it using:

```cmd
winget install T9ner.TermTalk
```

**Test the local manifest**:
You can install and test the local manifest directly using:
```cmd
winget install --manifest ./packaging/winget/T9ner.TermTalk.yaml
```

---

## Build & Run

### Prerequisites
- Go `1.26+`

### Quick Start

1. **Run Application**:
   ```bash
   go run cmd/termtalk/main.go
   ```

2. **Run Tests**:
   ```bash
   go test ./...
   ```

3. **Format Code**:
   ```bash
   go fmt ./...
   ```

4. **Lint Code**:
   ```bash
   go vet ./...
   ```

