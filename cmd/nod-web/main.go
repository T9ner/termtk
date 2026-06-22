// Command nod-web starts the Nod HTTP/WebSocket API server.
// The React web UI connects to this server over localhost.
//
// Usage:
//
//	nod-web [flags]
//	  -db string     Path to SQLite database file (default "./nod.db")
//	  -port int      TCP port for peer-to-peer messaging (default 55556)
//	  -api int       HTTP API port for the web UI (default 3001)
//	  -relay string  Relay server address (default "nod-relay.fly.dev:55558")
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nod/internal/client"
	"nod/internal/server"
)

func main() {
	dbFlag := flag.String("db", "", "Path to SQLite database file")
	tcpPortFlag := flag.Int("port", 55556, "TCP port for direct peer messaging")
	apiPortFlag := flag.Int("api", 3001, "HTTP API port for the web UI")
	relayFlag := flag.String("relay", "nod-relay.fly.dev:55558", "Address of the Nod relay server")
	flag.Parse()

	// Resolve database path.
	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = filepath.Join(".", "nod.db")
	}

	// Initialize Client.
	c, err := client.New(dbPath, *tcpPortFlag)
	if err != nil {
		log.Fatalf("Fatal: failed to initialize client: %v", err)
	}
	defer c.Stop()

	c.SetRelayAddr(*relayFlag)

	// Load profile (may be nil on first boot).
	p, err := c.LoadProfile()
	if err != nil {
		log.Fatalf("Fatal: failed to load profile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start networking if profile exists.
	if p != nil {
		if err := c.Start(ctx); err != nil {
			log.Fatalf("Fatal: failed to start networking: %v", err)
		}
		log.Printf("Profile loaded: %s (%s)", p.Username, p.UUID[:8])
	} else {
		log.Println("No profile found — register via POST /api/register")
	}

	// Start HTTP/WebSocket server.
	srv := server.New(c, *apiPortFlag)
	srv.SetStartNetworkingFunc(func() error {
		return c.Start(ctx)
	})
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("Fatal: failed to start API server: %v", err)
	}

	fmt.Printf("\n  Nod API server running\n")
	fmt.Printf("  ──────────────────────\n")
	fmt.Printf("  REST API:   http://127.0.0.1:%d/api\n", *apiPortFlag)
	fmt.Printf("  WebSocket:  ws://127.0.0.1:%d/ws\n", *apiPortFlag)
	fmt.Printf("  Health:     http://127.0.0.1:%d/api/health\n\n", *apiPortFlag)

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Stop(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	cancel()
	log.Println("Goodbye.")
}
