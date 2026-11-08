//go:build demo

// Demo builds (go build -tags demo, or scripts/build-app.sh --demo) run
// on a simulated card and SM-DP+ instead of real devices. Release builds
// leave this file, and the simulator, out.

package main

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/demo"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/locales"
)

const demoBuild = true

// useDemo points the app at the simulated card, which shows as a reader
// to choose in Settings, as a real one would.
func useDemo(opts *app.Options) {
	card := demo.New()
	opts.NewService = func(_ *settings.Config, onStage func(string)) (lpa.Service, error) {
		return card.Service(onStage), nil
	}
	opts.Discover = func(ctx context.Context, backend string) ([]transport.Device, error) {
		if backend != settings.BackendPCSC {
			return nil, nil
		}
		return demo.Discover(ctx, backend)
	}
}

// demoCodes offers the simulated scenarios' activation codes.
func (g *gui) demoCodes(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Padding(10, 12).Gap(4).Radius(12).Background(t.Accent.Alpha(0.08)).Border(1, t.Accent.Alpha(0.3)).Children(func() {
		ui.Text(c, locales.DlDemoCodes()).FontSize(12).Bold()
		for _, sc := range demo.Scenarios {
			row := ui.ButtonBase(c.Key(sc.Code)).Padding(4, 6).Gap(8).Radius(6).Label(sc.Description)
			if row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				ui.Text(c, sc.Description).FontSize(12).Grow(1).MinWidth(0)
				ui.Text(c, sc.Code[strings.LastIndex(sc.Code, "$DEMO")+1:]).FontSize(11).Font("monospace").TextColor(t.TextMuted)
			})
			if row.Clicked() {
				g.dl.code, g.dl.confirm, g.dl.inputError = sc.Code, "", ""
			}
		}
	})
}
