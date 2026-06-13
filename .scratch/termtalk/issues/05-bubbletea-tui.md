Status: completed

# Issue 5: Bubble Tea Terminal TUI

## What to build

Build a gorgeous, interactive terminal user interface using Charm.sh's `bubbletea`, `lipgloss`, and `bubbles`. The interface must support profile registration on first launch, a layout containing a contacts list, a scrollable messaging history panel, a message entry box, and controls for exporting/importing sync files.

## Acceptance criteria

- [ ] Interactive layout built with `bubbletea` Elm architecture (Model, Update, View).
- [ ] Visual style implemented with `lipgloss` (supporting dark/light theme adapting to terminal, borders, clean padding, and color accents).
- [ ] On first launch: render a registration screen to prompt the user for their username and generate their UUID.
- [ ] Main view:
  - Sidebar: List of contacts/friends with online/offline indicators (based on discovery status).
  - Chat pane: Message history with the selected contact, including scrollable view (using `bubbles/viewport`) and status indicators ("draft", "sent", "synced").
  - Input area: Text input (using `bubbles/textinput`) to type and send messages.
  - Action commands/keys: Keyboard shortcuts shown at the bottom of the screen (e.g. `Ctrl+E` to export sync, `Ctrl+I` to import sync, `Ctrl+Q` to quit).
- [ ] Integration: UI updates dynamically when P2P UDP discovery spots a peer or TCP sync updates messages.
- [ ] Executable compiles cleanly on macOS and Windows to a standalone binary.

## Blocked by

- [Issue 3: Direct Messaging & TCP Sync Protocol](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/03-tcp-sync.md)
- [Issue 4: Sneakernet Offline Sync File Export/Import](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/04-sneakernet-sync.md)
