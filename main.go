package main

import (
	"embed"
	"os"
	"runtime"

	"gcetools/internal/cli"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// `gcetools scan ...` runs headless (Linux/macOS; on Windows use gcetools-cli).
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		os.Exit(cli.Run(os.Args[2:]))
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		os.Exit(cli.Version())
	}

	app := NewApp()

	// macOS: without an App and Edit menu, Cmd+C/V/X/A and Cmd+Q do not work.
	var appMenu *menu.Menu
	if runtime.GOOS == "darwin" {
		appMenu = menu.NewMenu()
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}

	err := wails.Run(&options.App{
		Title:            "gceTools",
		Width:            1440,
		Height:           900,
		MinWidth:         1024,
		MinHeight:        680,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 253, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Menu:             appMenu,
		Mac: &mac.Options{
			About: &mac.AboutInfo{Title: "gceTools", Message: "Inventario de Compute Engine al estilo RVTools"},
		},
		Windows: &windows.Options{
			Theme: windows.SystemDefault, // title bar follows the OS light/dark setting
		},
	})
	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}
