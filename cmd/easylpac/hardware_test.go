package main

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/settings"
)

// TestHardwareReadOnly drives the GUI against a real PC/SC reader, read
// only:
//
//	EASYLPAC_HW_READER="<reader>" go test ./cmd/easylpac -run Hardware -v
func TestHardwareReadOnly(t *testing.T) {
	reader := os.Getenv("EASYLPAC_HW_READER")
	if reader == "" {
		t.Skip("set EASYLPAC_HW_READER to run against a real reader")
	}
	f := &fixture{g: newGUI(&memClipboard{}), q: make(chan func(), 256)}
	cfg := settings.Default()
	cfg.HideIdentifiers = true
	f.g.app = app.New(app.Options{
		Settings: cfg,
		Dispatch: func(fn func()) { f.q <- fn },
		Version:  "hw-test",
	})
	t.Cleanup(func() {
		if t.Failed() {
			entries, _ := f.g.app.Log.Snapshot()
			for _, e := range entries {
				t.Log(e.Text(true))
			}
		}
	})
	f.tt = ui.NewTester(f.g.view, 1040, 700)
	f.tt.SetTitleBar(ui.TitleBar{Height: titleBarHeight, Left: 76})
	f.tt.Frame()
	f.chooseReader(t, reader)
	if !f.g.app.Loaded {
		t.Fatalf("card not loaded: err=%v texts=%q", f.g.app.LastError, f.tt.Texts())
	}
	t.Logf("EID %s, %d profiles, %d notifications", app.Mask(f.g.app.Chip.EidValue), len(f.g.app.Profiles), len(f.g.app.Notifications))
	f.shot(t, "hw-profiles")
	f.click(t, "Card")
	f.tt.Frame()
	f.click(t, "Expand all")
	f.tt.Frame()
	t.Logf("EID check digits valid: %v", app.ValidEID(f.g.app.Chip.EidValue))
	f.shot(t, "hw-card")
	f.click(t, "Log")
	f.tt.Frame()
	f.shot(t, "hw-log")
	entries, _ := f.g.app.Log.Snapshot()
	for _, e := range entries {
		t.Log(e.Text(true))
	}
}
