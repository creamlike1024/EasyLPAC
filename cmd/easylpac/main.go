// Command easylpac is the EasyLPAC desktop app: an LPA for eUICC cards
// with libeuicc linked in, its interface drawn by MyGo.
package main

import (
	"log"

	"github.com/egoist/mygo"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/settings"
)

// Version is set at build time (-ldflags "-X main.Version=...").
var Version = "development"

func main() {
	isDemo := demoBuild
	if v := mygo.App.Version(); v != "" {
		Version = v
	}
	// Only the language is kept between launches, by MyGo (SetLocale);
	// every launch starts from the default settings and appearance, with
	// no device chosen.
	opts := app.Options{Version: Version, Settings: settings.Default()}
	if isDemo {
		useDemo(&opts)
	} else {
		opts.Settings.Backend = app.SupportedBackends[0]
	}

	g := newGUI(mygo.Clipboard)
	g.demo = isDemo
	title := "EasyLPAC"
	if isDemo {
		title = "EasyLPAC (Demo)"
	}
	mygo.App.WhenReady(func() {
		g.locale = chosenLocale()
		g.openWindows(title)
		opts.Dispatch = g.update
		g.app = app.New(opts)
		if isDemo {
			g.app.Logger.Info("demo mode: simulated card and SM-DP+")
		}
		g.app.Log.OnAppend(g.invalidate)
		// One look for devices at launch; after that only when asked.
		g.app.RefreshDevices()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
