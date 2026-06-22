package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"nod/internal/client"
	"nod/internal/ui"
)

func main() {
	// 1. Parse CLI arguments
	dbFlag := flag.String("db", "", "Path to SQLite database file")
	tcpPortFlag := flag.Int("port", 55556, "TCP port to listen on for direct peer messaging")
	relayFlag := flag.String("relay", "nod-relay.fly.dev:55558", "Address of the Nod relay server")
	flag.Parse()

	// 2. Resolve database path
	dbPath := *dbFlag
	if dbPath == "" {
		// Use current working directory for dev simplicity
		dbPath = filepath.Join(".", "nod.db")
	}

	// 3. Initialize Client Core
	c, err := client.New(dbPath, *tcpPortFlag)
	if err != nil {
		log.Fatalf("Fatal: failed to initialize client: %v", err)
	}
	defer c.Stop()

	// Set relay address
	c.SetRelayAddr(*relayFlag)

	// 4. Retrieve Profile (if registered)
	profile, err := c.LoadProfile()
	if err != nil {
		log.Fatalf("Fatal: database read error: %v", err)
	}

	if profile != nil {
		// Start networking if profile exists
		if err := c.Start(context.Background()); err != nil {
			log.Printf("Warning: failed to start client networking: %v", err)
		}
	}

	// 5. Run the Bubble Tea UI Loop
	model := ui.NewModel(c)
	if profile != nil {
		model.LocalUser = profile
		model.State = ui.StateDashboard
		model.RefreshContacts()
	} else {
		model.State = ui.StateRegister
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running Nod: %v\n", err)
		os.Exit(1)
	}
}
