// Command easylpac is the EasyLPAC desktop app: an LPA for eUICC cards
// with libeuicc linked in, its interface drawn by MyGo.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/settings"
)

// Version is set at build time (-ldflags "-X main.Version=...").
var Version = "development"

func main() {
	isDemo := demoBuild
	// Nothing is read from or written to disk: every launch starts from
	// the defaults, with no device chosen.
	opts := app.Options{Version: Version, Settings: settings.Default()}
	if isDemo {
		useDemo(&opts)
	}

	if v := mygo.App.Version(); v != "" {
		Version = v
	}
	g := newGUI(mygo.Clipboard)
	g.demo = isDemo
	title := "EasyLPAC"
	if isDemo {
		title = "EasyLPAC (Demo)"
	}
	mygo.App.WhenReady(func() {
		g.win = mygo.NewWindow(mygo.WindowOptions{
			Title:          title,
			Width:          1040,
			Height:         700,
			MinWidth:       760,
			MinHeight:      520,
			TitleBarStyle:  mygo.TitleBarHiddenInset,
			TitleBarHeight: titleBarHeight,
			Vibrancy:       mygo.VibrancySidebar,
			Content:        ui.View(g.view),
		})
		opts.Dispatch = g.win.Update
		g.app = app.New(opts)
		if isDemo {
			g.app.Logger.Info("demo mode: simulated card and SM-DP+")
		}
		g.app.Log.OnAppend(func() { g.win.Invalidate() })
		// One look for devices at launch; after that only when asked.
		g.app.RefreshDevices()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
