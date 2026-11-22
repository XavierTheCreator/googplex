package main

import (
	"embed"
	"log"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if slices.Contains(os.Args[1:], "--pip") {
		if err := runPip(os.Args[1:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	app := NewApp()
	theme := storedTheme()
	err := wails.Run(&options.App{
		Title:            "Googleplex",
		Width:            1240,
		Height:           820,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: windowColour(theme),
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app, &Pip{}}, // Pip is bound only so its JS bindings are generated
		// Files dropped on an element with --wails-drop-target: drop arrive as paths
		// (Drive uploads); the webview itself never opens a dropped file.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: macAppearance(theme),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
