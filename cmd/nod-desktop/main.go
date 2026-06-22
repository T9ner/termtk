package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailswindows "github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// windowState persists the user's window size and position across sessions.
type windowState struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

// loadWindowState reads saved window state from disk.
func loadWindowState(dataDir string) *windowState {
	path := filepath.Join(dataDir, "window_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var ws windowState
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil
	}
	// Sanity check: don't restore absurd values
	if ws.Width < 400 || ws.Height < 300 || ws.Width > 4000 || ws.Height > 3000 {
		return nil
	}
	return &ws
}

// saveWindowState writes current window state to disk.
func saveWindowState(dataDir string, ws windowState) {
	path := filepath.Join(dataDir, "window_state.json")
	data, _ := json.Marshal(ws)
	_ = os.WriteFile(path, data, 0o644)
}

func main() {
	// Resolve data directory: %APPDATA%\Nod\ (or NOD_DATA_DIR env)
	dataDir := os.Getenv("NOD_DATA_DIR")
	if dataDir == "" {
		appData, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("cannot resolve config dir: %v", err)
		}
		dataDir = filepath.Join(appData, "Nod")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "nod.db")

	// Create the app binding
	app := NewApp(dbPath, dataDir)

	// Load persisted window state (D5)
	width, height := 1100, 750
	ws := loadWindowState(dataDir)
	if ws != nil {
		width = ws.Width
		height = ws.Height
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Nod",
		Width:     width,
		Height:    height,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnDomReady: func(ctx context.Context) {
			// Restore window position if saved (D5)
			if ws != nil && ws.X >= 0 && ws.Y >= 0 {
				wailsruntime.WindowSetPosition(ctx, ws.X, ws.Y)
			}
		},
		Bind: []interface{}{
			app,
		},
		// Single instance lock — prevent duplicate Nod windows
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "nod-desktop-8f3a-4b2e-9d1c",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				wailsruntime.WindowUnminimise(app.ctx)
				wailsruntime.Show(app.ctx)
			},
		},
		Windows: &wailswindows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		fmt.Println("Error:", err.Error())
	}
}
