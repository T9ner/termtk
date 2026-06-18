package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailswindows "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Resolve data directory: %APPDATA%\TermTalk\ (or TERMTALK_DATA_DIR env)
	dataDir := os.Getenv("TERMTALK_DATA_DIR")
	if dataDir == "" {
		appData, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("cannot resolve config dir: %v", err)
		}
		dataDir = filepath.Join(appData, "TermTalk")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "termtalk.db")

	// Create the app binding
	app := NewApp(dbPath, dataDir)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "TermTalk",
		Width:     1100,
		Height:    750,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
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
