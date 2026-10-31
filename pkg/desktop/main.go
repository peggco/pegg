//go:build gui

package main

import (
	"embed"
	"log"

	"github.com/peggco/pegg/pkg/desktop/internal/prefs"
	"github.com/peggco/pegg/pkg/desktop/internal/themes"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/appicon.png
var appIcon []byte

func main() {
	app := NewApp()
	window := prefs.Window{Width: 1280, Height: 840}
	dark := false
	if store, err := prefs.Load(); err == nil {
		p := store.Get()
		if p.Window.Width > 0 {
			window.Width = p.Window.Width
		}
		if p.Window.Height > 0 {
			window.Height = p.Window.Height
		}
		if th, ok := themes.Find(p.Theme); ok {
			dark = th.Dark
		}
	}

	opts := &options.App{
		Title:            "Pegg",
		Width:            window.Width,
		Height:           window.Height,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		AlwaysOnTop:      false,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind:             []interface{}{app},
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Linux: &linux.Options{
			Icon:             appIcon,
			ProgramName:      "pegg",
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
	}
	applyPlatformOptions(opts, dark)

	if err := wails.Run(opts); err != nil {
		log.Fatal(err)
	}
}
