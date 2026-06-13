package main

import (
	"embed"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var appVersion = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "PGP Manager",
		Width:            1100,
		Height:           680,
		MinWidth:         800,
		MinHeight:        520,
		MaxWidth:         1920,
		MaxHeight:        1080,
		// No Frameless — that would remove macOS traffic lights entirely.
		// Mac.TitleBar: TitleBarHiddenInset() makes content fill the full window
		// while keeping native traffic lights; HTML drag region handles window moving.
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true},
		BackgroundColour: &options.RGBA{R: 28, G: 28, B: 30, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		// With "Run in System Tray" enabled the window only appears when
		// explicitly opened (tray menu, service action, auto-detect).
		// During first-run setup the window always shows.
		StartHidden: app.cfg.StartInTray && !app.needsSetup,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.developaaah.pgp-manager",
			OnSecondInstanceLaunch: app.handleSecondInstance,
		},
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
		Mac: &mac.Options{
			TitleBar:            mac.TitleBarHidden(),
			WindowIsTranslucent: true,
			OnUrlOpen:           func(url string) { app.handleActionRequest(url) },
			About: &mac.AboutInfo{
				Title:   "PGP Manager",
				Message: "Cross-platform PGP key management.",
			},
		},
		Windows: &windows.Options{},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "PGP Manager",
		},
		// Close behavior is dynamic (OnBeforeClose): tray mode hides the
		// window, non-tray mode quits the app.
	})
	if err != nil {
		slog.Error("wails run failed", "error", err)
	}
}
