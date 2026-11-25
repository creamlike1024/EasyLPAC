//go:build android || phone

package main

// The frame of phones: the system's bar of tabs, a window in each, and
// windows pushed over the settings tab's. The pages and the dialogs are
// gui.go's and their own files'. Tests build it on desktops with
// -tags phone.

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"slices"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/imagepicker"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"

	"EasyLPAC/locales"
)

// phoneUI is set where pages take the layout of phones.
const phoneUI = true

const (
	// phonePadding is the padding of a phone's page.
	phonePadding = 16
	// phoneActions is the height of a row of actions above a page, and the
	// gap below it.
	phoneActions = 36 + 16
	// frameChrome is what the frame takes from the log page: its paddings
	// and the row of actions above it.
	frameChrome = 2*phonePadding + phoneActions
	// tabIconSize is the size of the tabs' icons, in pixels: sharp at
	// every phone's scale.
	tabIconSize = 72
)

// phoneTab is a tab of the bar: the page it shows.
type phoneTab struct {
	page  string
	title func() string
	icon  *ui.SVG
}

var phoneTabs = []phoneTab{
	{pageProfiles, locales.NavEsims, iconSIM},
	{pageNotifications, locales.NavNotifications, iconBell},
	{pageCard, locales.NavCard, iconChip},
	{pageSettings, locales.NavSettings, iconSettings},
}

// phoneState is the frame's own: what the tabs and stack show.
type phoneState struct {
	tabs *mygo.Tabs
	// pushed are the pages pushed over the settings tab, the last in
	// front.
	pushed []string
	// pull is the pull to refresh of each page that reads the card.
	pull map[string]*ui.RefreshState
	// badge is the notifications tab's badge, as last set.
	badge string
	// dialogTab is the page of the tab a dialog opened on, while it is
	// open: it stays on that tab.
	dialogTab string
}

// openWindows shows the bar of tabs, a window of a page in each.
func (g *gui) openWindows(string) {
	items := make([]mygo.TabItem, len(phoneTabs))
	for i, tab := range phoneTabs {
		w := mygo.NewWindow(mygo.WindowOptions{Title: tab.title(), Hidden: true, Content: ui.View(g.pageView(tab.page))})
		g.wins = append(g.wins, w)
		items[i] = mygo.TabItem{Title: tab.title(), Icon: tabIcon(tab.icon), Window: w}
	}
	g.win = g.wins[0]
	tabs, err := mygo.NewTabs(items...)
	if err != nil {
		log.Fatal(err)
	}
	// The tabs make the windows' bars: their buttons go in after.
	g.setBarItems()
	// The bars say the language chosen in Settings, or the system's.
	// The tab bar's own labels stay those of the launch: MyGo's Tabs
	// can't change them.
	mygo.App.OnLocaleChanged(func(string) { g.retitle() })
	g.phone.tabs = tabs
	tabs.OnSelect(func(i int) {
		if back := g.tabChosen(phoneTabs[i].page); back != "" {
			tabs.Select(tabIndex(back))
		}
		g.invalidate()
	})
	g.selectTab = func(page string) {
		if i := tabIndex(page); i >= 0 {
			tabs.Select(i)
			g.invalidate()
		}
	}
	g.invalidate()
}

// shownWins are the windows on screen: the chosen tab's, or the page
// pushed last over the settings tab, while that tab is chosen.
func (g *gui) shownWins() []*mygo.Window {
	i := tabIndex(g.page)
	if i < 0 || i >= len(g.wins) {
		return nil
	}
	if g.page == pageSettings && len(g.phone.pushed) > 0 && len(g.wins) > len(phoneTabs) {
		return g.wins[len(g.wins)-1:]
	}
	return g.wins[i : i+1]
}

// tabChosen makes page's tab the one shown, as the user chose it, unless
// a dialog is open on another tab: a dialog keeps the screen to itself,
// as Android's do, and the tab to go back to is returned.
func (g *gui) tabChosen(page string) (back string) {
	if g.dialogOpen() && g.phone.dialogTab != "" && g.phone.dialogTab != page {
		return g.phone.dialogTab
	}
	g.page = page
	return ""
}

// tabIndex is the index of page's tab, -1 if it has none.
func tabIndex(page string) int {
	return slices.IndexFunc(phoneTabs, func(t phoneTab) bool { return t.page == page })
}

// readsCard reports whether page shows what is read from the card, and
// so refreshes.
func readsCard(page string) bool {
	return page == pageProfiles || page == pageNotifications || page == pageCard
}

// tabIcon draws an icon as a PNG for a bar, which draws its alpha.
func tabIcon(s *ui.SVG) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, s.Image(tabIconSize, tabIconSize, color.Black)); err != nil {
		panic(err) // encoding an image in memory does not fail
	}
	return b.Bytes()
}

// front reports whether page is the one in front, which takes what is
// shown once: toasts, dialogs.
func (g *gui) front(page string) bool {
	// The pages pushed over the settings tab cover it alone: on another
	// tab, that tab's own page is in front.
	if n := len(g.phone.pushed); n > 0 && g.page == pageSettings {
		return g.phone.pushed[n-1] == page
	}
	return g.page == page
}

// pageView builds the window of a page, a tab's or one pushed over it,
// every frame.
func (g *gui) pageView(page string) func(*ui.Context) {
	return func(c *ui.Context) {
		if g.app == nil {
			return
		}
		g.phoneFrame(c, page)
	}
}

// phoneFrame is page in its window: scrolled, with its actions above it.
func (g *gui) phoneFrame(c *ui.Context, page string) {
	t := c.Theme()
	c.Root().Background(t.Background)
	// The tab a dialog opens on is the one in front.
	if !g.dialogOpen() {
		g.phone.dialogTab = ""
	} else if g.phone.dialogTab == "" {
		g.phone.dialogTab = g.page
	}
	front := g.front(page)
	if front {
		g.showNotices(c)
		g.syncBadge()
	}
	bottom := float32(phonePadding)
	if g.app.Task != nil {
		bottom = 76 // the task pill floats there
	}
	// The button would show over a dialog's backdrop: it goes while one
	// is open.
	fab := page == pageProfiles && g.app.Loaded && !g.dialogOpen()
	pad, right := bottom, float32(phonePadding)
	if fab {
		// Upright, the last eSIM scrolls above the button. Sideways the
		// page is too short for that, and the button would cover the rows'
		// ends: it gets a margin of its own on the right.
		if w, h := c.Size(); w > h {
			right += 56 + phonePadding
		} else {
			pad += 56 + phonePadding
		}
	}
	list := ui.Scroll(c).Fill().Padding(phonePadding, right, pad, phonePadding).Gap(16)
	if readsCard(page) {
		pull := g.pullState(page)
		if list.PullToRefresh(pull) {
			g.pulled(pull)
		}
		if pull.Refreshing && !g.app.Busy() {
			pull.Refreshing = false // the read is over
		}
	}
	list.Children(func() {
		g.phoneActions(c, page)
		g.pageBody(c, page)
		if page == pageSettings {
			g.morePages(c)
		}
	})
	if fab {
		// Placed in the root's padding box, which takes in what covers
		// the window's edges (a navigation bar at a side): it keeps clear
		// of that as the page does.
		_, sr, sb, _ := c.SafeArea()
		ui.Box(c).Absolute().Right(phonePadding + sr).Bottom(bottom + sb).Children(func() {
			fab := mobile.FloatingButton(c, locales.ActionAddEsim(), nil).Center().Children(func() {
				ui.Icon(c, iconPlus).Size(24, 24).TextColor(t.AccentText)
			})
			if fab.Clicked() {
				g.openDownload()
			}
		})
	}
	if front {
		g.dialogs(c)
	}
	g.taskPill(c)
}

// dialogOpen reports whether a dialog of the pages is open.
func (g *gui) dialogOpen() bool {
	return g.dl.open || g.renaming || g.deleting || g.certOpen || g.editSMDP || g.removeAll
}

// pulled reads the card again as a finger asked, or sends the indicator
// away when it can't.
func (g *gui) pulled(pull *ui.RefreshState) {
	if g.canRefresh() {
		g.refresh()
	} else {
		pull.Refreshing = false
	}
}

func (g *gui) pullState(page string) *ui.RefreshState {
	if g.phone.pull == nil {
		g.phone.pull = map[string]*ui.RefreshState{}
	}
	s := g.phone.pull[page]
	if s == nil {
		s = &ui.RefreshState{}
		g.phone.pull[page] = s
	}
	return s
}

// phoneActions is the row of a page's actions, which desktops have in the
// header.
func (g *gui) phoneActions(c *ui.Context, page string) {
	var actions func(*ui.Context)
	switch page {
	case pageNotifications:
		if !g.app.Loaded {
			return
		}
		actions = g.notificationActions
	case pageLog:
		actions = g.logActions
	default:
		return
	}
	ui.Row(c).Height(36).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() { actions(c) })
}

// morePages lists the pages the settings tab leads to.
func (g *gui) morePages(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Radius(14).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
		for i, page := range []string{pageLog, pageAbout} {
			if i > 0 {
				ui.Divider(c)
			}
			row := mobile.ListRow(c.Key(page), mobile.RowOptions{Title: pageTitle(page), Disclosure: true})
			if row.Clicked() {
				g.push(page)
			}
		}
	})
}

// setBarItems puts the refresh button in the bars of the tabs that read
// the card.
func (g *gui) setBarItems() {
	refresh := mygo.BarItem{Title: locales.ActionRefresh(), Icon: tabIcon(iconRefresh), OnClick: g.refresh}
	for i, tab := range phoneTabs {
		if readsCard(tab.page) {
			mygo.SetBarItems(g.wins[i], refresh)
		}
	}
}

// retitle says the windows' titles, and their bars' buttons, in the
// language now shown.
func (g *gui) retitle() {
	if len(g.wins) < len(phoneTabs) {
		return
	}
	for i, tab := range phoneTabs {
		g.wins[i].SetTitle(tab.title())
	}
	// The pushed windows follow the tabs', in the order pushed.
	for i, page := range g.phone.pushed {
		if j := len(phoneTabs) + i; j < len(g.wins) {
			g.wins[j].SetTitle(pageTitle(page))
		}
	}
	g.setBarItems()
}

// push shows page in a window over the settings tab's, which the
// system's back button takes away.
func (g *gui) push(page string) {
	if slices.Contains(g.phone.pushed, page) {
		return // a second tap before it slid in
	}
	g.phone.pushed = append(g.phone.pushed, page)
	if len(g.wins) == 0 {
		return // tests have no windows
	}
	w := mygo.NewWindow(mygo.WindowOptions{Title: pageTitle(page), Hidden: true, Content: ui.View(g.pageView(page))})
	g.wins = append(g.wins, w)
	w.OnClosed(func() {
		g.wins = slices.DeleteFunc(g.wins, func(x *mygo.Window) bool { return x == w })
		if i := slices.Index(g.phone.pushed, page); i >= 0 {
			g.phone.pushed = g.phone.pushed[:i]
		}
		g.invalidate()
	})
	g.wins[tabIndex(pageSettings)].Push(w)
	g.invalidate()
}

// syncBadge counts the notifications on their tab.
func (g *gui) syncBadge() {
	badge := ""
	if n := len(g.app.Notifications); g.app.Loaded && n > 0 {
		badge = fmt.Sprint(n)
	}
	if badge == g.phone.badge || g.phone.tabs == nil {
		return
	}
	g.phone.badge = badge
	g.phone.tabs.SetBadge(tabIndex(pageNotifications), badge)
}

// phoneQRButtons read an activation code's QR code with the system's
// camera, or from a photo the system's picker gives: neither needs a
// permission.
func (g *gui) phoneQRButtons(c *ui.Context) {
	if ui.Button(c, "").Radius(14).Children(func() {
		ui.Icon(c, iconCamera).Size(16, 16)
		ui.Text(c, locales.DlScanQr()).SingleLine()
	}).Clicked() {
		go func() {
			path, err := imagepicker.Capture(context.Background())
			g.scanQRPhoto(path, err)
		}()
	}
	if ui.Button(c, "").Radius(14).Children(func() {
		ui.Icon(c, iconImage).Size(16, 16)
		ui.Text(c, locales.DlChoosePhoto()).SingleLine()
	}).Clicked() {
		go func() {
			paths, err := imagepicker.Pick(context.Background(), imagepicker.Options{})
			for _, p := range paths[min(1, len(paths)):] {
				_ = os.Remove(p) // one is asked for
			}
			path := ""
			if len(paths) > 0 {
				path = paths[0]
			}
			g.scanQRPhoto(path, err)
		}()
	}
}
