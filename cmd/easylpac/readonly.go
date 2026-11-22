package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/settings"
)

// readOnlyLogLines is how much of the log the reader test shows.
const readOnlyLogLines = 16

// readOnlyView tests a USB CCID reader on a phone, until the app has an
// interface for phones. It only reads the card: it has no action that
// writes to it, and it always obscures the identifiers, as it is there to
// be captured in screenshots. A development screen, it is not translated.
func (g *gui) readOnlyView(c *ui.Context) {
	if g.app == nil {
		return
	}
	a, t := g.app, c.Theme()
	a.TakeNotices() // the screen shows the outcome itself
	c.Root().Background(t.Background)
	busy := a.Busy()

	ui.Scroll(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Text(c, "Reader test (read only)").FontSize(18).Bold()

		card(c, func() {
			if len(a.Devices) == 0 {
				msg := "No reader found. Plug one in, then find readers."
				if a.Discovering {
					msg = "Looking for readers..."
				}
				ui.Text(c, msg).TextColor(t.TextMuted)
			}
			for _, d := range a.Devices {
				ui.Text(c, d.Label)
				ui.Text(c, d.ID).FontSize(12).TextColor(t.TextMuted)
				if ui.PrimaryButton(c.Key(d.ID), "Read card").Disabled(busy).Clicked() {
					if d.ID == a.CurrentDevice() {
						_ = a.Refresh()
					} else {
						a.SelectDevice(d.ID)
					}
				}
			}
			if ui.Button(c, "Find readers").Disabled(busy || a.Discovering).Clicked() {
				a.RefreshDevices()
			}
			b := a.Settings.Backends[settings.BackendCCID]
			pps := !b.NoPPS
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, "Faster card rate (PPS)").Grow(1).MinWidth(0)
				if ui.Switch(c, &pps).Label("Faster card rate (PPS)").Disabled(busy).Changed() {
					a.UpdateSettings(func(s *settings.Config) {
						bb := s.Backends[settings.BackendCCID]
						bb.NoPPS = !pps
						s.Backends[settings.BackendCCID] = bb
					})
				}
			})
		})

		switch {
		case busy:
			card(c, func() {
				ui.Text(c, "Reading. Allow USB access if asked.")
				if ui.Button(c, "Cancel").Clicked() {
					a.Task.Cancel()
				}
			})
		case a.LastError != nil:
			card(c, func() {
				ui.Text(c, "Failed").Bold().TextColor(t.Danger)
				ui.Text(c, app.DescribeError(a.LastError)).FontSize(12)
			})
		case a.Loaded:
			card(c, func() {
				ui.Text(c, "EID "+app.Mask(a.Chip.EidValue))
				ui.Text(c, "Profiles: "+strconv.Itoa(len(a.Profiles)))
				for _, p := range a.Profiles {
					ui.Text(c, app.Mask(p.Iccid)+" "+p.ProfileState+" "+p.ServiceProviderName).FontSize(12)
				}
				ui.Text(c, "Notifications: "+strconv.Itoa(len(a.Notifications)))
			})
		}

		entries, _ := a.Log.Snapshot()
		for _, e := range entries[max(0, len(entries)-readOnlyLogLines):] {
			ui.Text(c, e.Text(true)).FontSize(10).Font("monospace").TextColor(t.TextMuted)
		}
	})
}
