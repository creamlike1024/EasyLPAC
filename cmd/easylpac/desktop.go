package main

// The frame of desktop windows: a sidebar, and the page under a floating
// header. The pages and the dialogs are gui.go's and their own files'.

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/locales"
)

const (
	sidebarWidth = 232
	headerHeight = 48
	// desktopChrome is what the floating header (68) and the bottom
	// padding (28) take from a page.
	desktopChrome = 68 + 28
)

// view builds the window from the state, every frame.
func (g *gui) view(c *ui.Context) {
	if g.app == nil {
		return
	}
	t := c.Theme()
	g.showNotices(c)

	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(t.Background)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		side := ui.Column(c).Width(sidebarWidth).Shrink(0)
		if !vibrant {
			side.Background(t.Surface)
		}
		side.Children(func() { g.sidebar(c, vibrant) })
		ui.Box(c).Width(1).Background(t.Border)
		ui.Box(c).Grow(1).MinWidth(0).Background(t.Background).Clip().Children(func() {
			g.pane(c)
		})
	})
	g.dialogs(c)

	// Cmd/Ctrl+R reads the card again, Cmd/Ctrl+N adds an eSIM.
	if c.Shortcut(ui.Cmd, ui.KeyR) {
		g.refresh()
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && g.app.Loaded {
		g.openDownload()
	}
}

// sidebar shows the device and the pages.
func (g *gui) sidebar(c *ui.Context, vibrant bool) {
	ui.Box(c).Height(titleBarHeight).Shrink(0).DragWindow()
	g.deviceCard(c)
	nProfiles, nNotif := "", ""
	if g.app.Loaded {
		nProfiles = fmt.Sprint(len(g.app.Profiles))
		if n := len(g.app.Notifications); n > 0 {
			nNotif = fmt.Sprint(n)
		}
	}
	list := ui.Sidebar(c, &g.page, func() {
		ui.SidebarSection(c, locales.NavSectionEuicc(), nil, func() {
			item := ui.SidebarItem(c, pageProfiles, iconSIM, locales.NavEsims())
			if nProfiles != "" {
				item.Children(func() { ui.Badge(c, nProfiles) })
			}
			item = ui.SidebarItem(c, pageNotifications, iconBell, locales.NavNotifications())
			if nNotif != "" {
				item.Children(func() { ui.Badge(c, nNotif) })
			}
			ui.SidebarItem(c, pageCard, iconChip, locales.NavCard())
		})
		ui.SidebarSection(c, locales.NavSectionApp(), nil, func() {
			ui.SidebarItem(c, pageSettings, iconSettings, locales.NavSettings())
			ui.SidebarItem(c, pageLog, iconLog, locales.NavLog())
			ui.SidebarItem(c, pageAbout, iconInfo, locales.NavAbout())
		})
	}).Grow(1).Label(locales.NavPages())
	if vibrant {
		list.Background(ui.Transparent)
	}
}

// deviceCard shows which device is used and its state; it opens the
// connection settings.
func (g *gui) deviceCard(c *ui.Context) {
	t := c.Theme()
	name, status, color := g.deviceState(c)
	card := ui.ButtonBase(c).Margin(0, 12, 10).Padding(10, 12).Gap(10).Radius(12).Label(locales.DeviceLabel(name))
	if t.Dark {
		card.Material(glass.Glass{Clarity: 0.4})
	} else {
		// Glass over the light sidebar material reads as a gray smudge.
		card.Background(t.Background).Border(1, t.Border).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	}
	card.Children(func() {
		ui.Icon(c, deviceIcon(g.app.Settings.Backend)).Size(18, 18).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, name).FontSize(12).Bold().SingleLine().MinWidth(0)
				if g.demo {
					ui.Badge(c, locales.DeviceDemo())
				}
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.Box(c).Size(7, 7).Radius(4).Background(color)
				ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
		})
	})
	if card.Clicked() {
		g.show(pageSettings)
	}
}

// pane is the page under its floating glass header.
func (g *gui) pane(c *ui.Context) {
	bar := c.TitleBar()
	top := float32(8) + headerHeight + 12
	bottom := float32(28)
	if g.app.Task != nil {
		bottom = 76 // the task pill floats there
	}
	ui.Scroll(c).Fill().Padding(top, 28, bottom).Gap(16).ScrollbarInsets(top, 0, 0).Children(func() {
		g.pageBody(c, g.page)
	})
	// Content scrolls under the header, faded by the scroll edge.
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(top + 8).PassThrough().Material(glass.ScrollEdge{})
	header := ui.Row(c).Absolute().Top(8).Left(12).Right(bar.Right+12).Height(headerHeight).
		Radius(headerHeight/2).Padding(0, 8, 0, 18).Gap(6).Material(glass.Glass{}).DragWindow()
	header.Children(func() {
		ui.Text(c, pageTitle(g.page)).FontSize(15).Bold().SingleLine().Grow(1).MinWidth(0)
		g.headerActions(c)
	})
	g.taskPill(c)
}

func (g *gui) headerActions(c *ui.Context) {
	switch g.page {
	case pageProfiles, pageNotifications, pageCard:
		if iconButton(c, iconRefresh, locales.ActionRefresh()).Disabled(!g.canRefresh()).Clicked() {
			g.refresh()
		}
	}
	switch g.page {
	case pageProfiles:
		add := ui.PrimaryButton(c, "").Radius(16).Disabled(!g.app.Loaded || g.app.Busy()).Children(func() {
			ui.Icon(c, iconPlus).Size(16, 16)
			ui.Text(c, locales.ActionAddEsim()).SingleLine()
		})
		if add.Clicked() {
			g.openDownload()
		}
	case pageNotifications:
		g.notificationActions(c)
	case pageLog:
		g.logActions(c)
	}
}
