package main

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
)

func TestReadOnlyView(t *testing.T) {
	fc := newFakeCard()
	cfg := settings.Default()
	cfg.Backend = settings.BackendCCID
	f := &fixture{g: newGUI(&memClipboard{}), q: make(chan func(), 256)}
	var got settings.Backend
	f.g.app = app.New(app.Options{
		Settings: cfg,
		Dispatch: func(fn func()) { f.q <- fn },
		Version:  "test",
		NewService: func(s *settings.Config, _ func(string)) (lpa.Service, error) {
			got = s.Backends[settings.BackendCCID]
			return fc, nil
		},
		Discover: func(context.Context, string) ([]transport.Device, error) {
			return []transport.Device{{ID: "/dev/bus/usb/001/002", Label: "ACS CCID USB Reader (072f:90cc)"}}, nil
		},
	})
	f.tt = ui.NewTester(f.g.readOnlyView, 360, 720)
	f.g.app.RefreshDevices()
	f.until(t, func() bool { return !f.g.app.Discovering })
	f.click(t, "Read card")
	f.until(t, func() bool { return f.g.app.Loaded })
	if got.Reader != "/dev/bus/usb/001/002" || got.NoPPS {
		t.Fatalf("backend settings %+v", got)
	}
	text := strings.Join(f.tt.Texts(), "\n")
	if !strings.Contains(text, "Profiles: 2") {
		t.Fatalf("no result shown: %q", text)
	}
	for _, id := range []string{f.g.app.Chip.EidValue, f.g.app.Profiles[0].Iccid, f.g.app.Profiles[1].Iccid} {
		if strings.Contains(text, id) {
			t.Errorf("identifier shown whole: %s", app.Mask(id))
		}
	}
}
