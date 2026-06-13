package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"termtalk/internal/db"
	"termtalk/internal/network"
	"termtalk/internal/ui"
)

func main() {
	// 1. Parse CLI arguments
	dbFlag := flag.String("db", "", "Path to SQLite database file")
	tcpPortFlag := flag.Int("port", 55556, "TCP port to listen on for direct peer messaging")
	flag.Parse()

	// 2. Resolve database path
	dbPath := *dbFlag
	if dbPath == "" {
		// Use current working directory for dev simplicity
		dbPath = filepath.Join(".", "termtalk.db")
	}

	// 3. Initialize SQLite Database
	database, err := db.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Fatal: failed to initialize database: %v", err)
	}
	defer database.Close()

	// 4. Retrieve Profile (if registered)
	profile, err := database.GetProfile()
	if err != nil {
		log.Fatalf("Fatal: database read error: %v", err)
	}

	// 5. Initialize background channels and managers
	eventChan := make(chan ui.MsgEvent, 100)

	var discovery *network.PeerDiscovery
	var syncMgr *network.SyncManager

	if profile != nil {
		// Profile exists, create and start networking services
		syncMgr = network.NewSyncManager(profile.UUID, profile.Username, *tcpPortFlag, database)
		discovery = network.NewPeerDiscovery(profile.UUID, profile.Username, *tcpPortFlag, database)

		if err := syncMgr.Start(); err != nil {
			log.Printf("Warning: failed to start TCP Sync Server: %v", err)
		} else {
			defer syncMgr.Stop()
		}

		if err := discovery.Start(); err != nil {
			log.Printf("Warning: failed to start UDP Discovery: %v", err)
		} else {
			defer discovery.Stop()
		}
	} else {
		// Profile doesn't exist yet (first boot), create skeleton managers that TUI will launch
		syncMgr = network.NewSyncManager("", "", *tcpPortFlag, database)
		discovery = network.NewPeerDiscovery("", "", *tcpPortFlag, database)
		defer func() {
			syncMgr.Stop()
			discovery.Stop()
		}()
	}

	// Bind callbacks to relay networking events into the TUI event loop
	discovery.OnPeerFound = func(contact *db.Contact) {
		eventChan <- ui.PeerDiscoveredMsg{Contact: contact}
	}

	syncMgr.OnMsgRecv = func(msg *db.Message) {
		eventChan <- ui.MessageReceivedMsg{Message: msg}
	}

	// 6. Run the Bubble Tea UI Loop
	model := ui.NewModel(database, discovery, syncMgr, eventChan)
	if profile != nil {
		model.LocalUser = profile
		model.State = ui.StateDashboard
		model.RefreshContacts()
	} else {
		model.State = ui.StateRegister
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TermTalk: %v\n", err)
		os.Exit(1)
	}
}
