//go:build demo

package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/demo"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
)

func TestDemoModeDownloadAndServerError(t *testing.T) {
	card := demo.New()
	card.Speed = 0
	f := &fixture{g: newGUI(&memClipboard{}), q: make(chan func(), 256)}
	f.g.demo = true
	cfg := settings.Default()
	cfg.AutoNotify = true
	f.g.app = app.New(app.Options{
		Settings:   cfg,
		Dispatch:   func(fn func()) { f.q <- fn },
		NewService: func(_ *settings.Config, onStage func(string)) (lpa.Service, error) { return card.Service(onStage), nil },
		Discover:   demo.Discover,
	})
	f.tt = ui.NewTester(f.g.view, 1040, 760)
	f.tt.SetTitleBar(ui.TitleBar{Height: titleBarHeight, Left: 76})
	f.tt.Frame()
	f.chooseReader(t, demo.ReaderName)
	if !f.tt.HasText("DEMO") || !f.tt.HasText("Travel 10 GB") {
		t.Fatalf("texts %q", f.tt.Texts())
	}
	f.shot(t, "demo-profiles")

	// A refused matching ID shows what the error code means.
	f.click(t, "Add eSIM")
	f.tt.Frame()
	f.click(t, "The server refuses the matching ID (8.2.6 / 3.8)")
	f.tt.Frame()
	f.shot(t, "demo-codes")
	f.click(t, "Download")
	f.until(t, func() bool { return !f.g.app.Busy() && f.g.dl.step == dlFailed })
	if !f.tt.HasText("Error code: subject 8.2.6, reason 3.8") {
		t.Fatalf("texts %q", f.tt.Texts())
	}
	f.shot(t, "demo-error")

	// Back to the form, a good code installs.
	f.click(t, "Edit and retry")
	f.tt.Frame()
	f.click(t, "Installs an eSIM")
	f.tt.Frame()
	f.click(t, "Download")
	f.until(t, func() bool { return f.g.app.Confirm != nil })
	f.click(t, "Install")
	f.until(t, func() bool { return !f.g.app.Busy() && f.g.dl.step == dlDone })
	if !f.tt.HasText("eSIM added") {
		t.Fatalf("texts %q", f.tt.Texts())
	}
}
