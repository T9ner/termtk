# 💬 TermTalk

An offline-first, peer-to-peer (P2P) terminal messaging application built in Go.

Whether you're connected to the internet, on a local Wi-Fi network with no internet access, or completely disconnected, **TermTalk** keeps you in touch with your friends. It compiles into a single, zero-dependency native binary for both **Windows** and **macOS** with no runtime requirements.

---

## ✨ Features

*   **⚡ P2P Local Discovery**: Periodically broadcasts presence over UDP. Peers on the same local network auto-detect each other's IP addresses and ports with zero configuration.
*   **🔄 TCP Sync Protocol**: Directly establishes TCP channels to exchange message logs. If you've been offline, TermTalk automatically swaps message histories to reconcile missing conversations.
*   **💾 Pure Go SQLite**: Persistence uses `github.com/ncruces/go-sqlite3`, a WebAssembly-compiled SQLite driver. It operates with **zero CGO requirements**, making cross-compilation painless.
*   **📦 Sneakernet Offline Sync**: Completely offline? Compose messages and queue them in your local database. Export them to a `.json` sync file, transfer it via a USB drive (or SD card), and import it on your friend's computer.
*   **🎨 Bubble Tea TUI**: A terminal user interface designed using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss) from Charm.sh.

---

## 🚀 Getting Started

### Prerequisites
*   [Go](https://go.dev/) 1.22+ (only required if building from source)

### Installation & Build

1.  Clone the repository:
    ```bash
    git clone https://github.com/yourusername/termtalk.git
    cd termtalk
    ```
2.  Compile the binary:
    *   **Windows**:
        ```bash
        go build -o termtalk.exe ./cmd/termtalk/main.go
        ```
    *   **macOS / Linux**:
        ```bash
        go build -o termtalk ./cmd/termtalk/main.go
        ```

### Cross-Compiling
Because TermTalk uses a pure-Go SQLite driver, you can cross-compile without a C cross-compiler:
*   Compile for macOS (Apple Silicon) from Windows:
    ```bash
    $env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o termtalk ./cmd/termtalk/main.go
    ```
*   Compile for Windows from macOS/Linux:
    ```bash
    GOOS=windows GOARCH=amd64 go build -o termtalk.exe ./cmd/termtalk/main.go
    ```

---

## 📖 How to Use

### Running Local Peers (Testing Setup)
To simulate two users chatting on the same computer:

1.  **Start Alice** on port `55556`:
    ```bash
    ./termtalk -db alice.db -port 55556
    ```
2.  **Start Bob** on port `55557`:
    ```bash
    ./termtalk -db bob.db -port 55557
    ```
3.  Choose usernames and complete the registration.
4.  Once registered, the instances will automatically discover each other over UDP broadcast and establish a direct connection.

### ⌨️ TUI Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `Up / Down` | Select contact from the sidebar |
| `Ctrl + N` | Add contact manually by pasting `username:uuid` |
| `Ctrl + E` | Export pending offline messages to a sync file (Sneakernet) |
| `Ctrl + I` | Import a sync file from a USB drive/file |
| `Ctrl + Q` | Quit application |

---

## 🏗️ Architecture

```text
termtalk/
├── cmd/
│   └── termtalk/
│       └── main.go         # App entry point
├── internal/
│   ├── db/
│   │   ├── db.go           # SQLite schema migrations & operations
│   │   ├── models.go       # Structs for Profiles, Contacts, Messages
│   │   └── sneakernet.go   # JSON sync file export/import
│   ├── network/
│   │   ├── discovery.go    # UDP broadcast peer discovery
│   │   └── sync.go         # TCP direct sync & message server
│   └── ui/
│       ├── model.go        # Bubble Tea main model
│       ├── update.go       # Event handling loop
│       └── view.go         # Lipgloss layouts & view rendering
```

---

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.
