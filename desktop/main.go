package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	appMenu := menu.NewMenu()

	// App menu (macOS) — built manually to include Settings
	appSubMenu := appMenu.AddSubmenu("Activate")
	appSubMenu.AddText("About Activate Framework", nil, nil)
	appSubMenu.AddSeparator()
	appSubMenu.AddText("Settings…", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "navigate", "settings")
	})
	appSubMenu.AddSeparator()
	appSubMenu.AddText("Hide Activate", keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) {
		runtime.Hide(app.ctx)
	})
	appSubMenu.AddSeparator()
	appSubMenu.AddText("Quit Activate", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(app.ctx)
	})

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Workspace…", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "navigate", "browse")
	})

	// View menu
	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Usage", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "navigate", "usage")
	})

	// Edit menu (standard copy/paste/undo)
	appMenu.Append(menu.EditMenu())

	err := wails.Run(&options.App{
		Title:  "Activate Framework",
		Width:  480,
		Height: 720,
		Menu:   appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarDefault(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
