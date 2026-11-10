package main

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/registry"
	"EasyLPAC/internal/settings"
	"EasyLPAC/locales"
)

const (
	titleBarHeight = 52
	sidebarWidth   = 232
	headerHeight   = 48
)

// The pages, which are the sidebar's ids.
const (
	pageProfiles      = "profiles"
	pageNotifications = "notifications"
	pageCard          = "card"
	pageSettings      = "settings"
	pageLog           = "log"
	pageAbout         = "about"
)

// gui is the state of the window: what the app layer does not own.
type gui struct {
	win  *mygo.Window
	app  *app.App
	page string

	// Profiles page.
	renaming    bool
	renameICCID string
	renameText  string
	deleting    bool
	deleteICCID string
	icons       map[string]ui.ImageSource

	// Notifications page.
	notifList ui.ListState
	notifSel  ui.Selection[int]
	notifSort ui.SortOrder
	removeAll bool

	// Card page: the default SM-DP+ dialog, and EUICCInfo2 as JSON, its
	// tree and which of its nodes are open.
	editSMDP bool
	smdpText string
	infoJSON string
	infoTree *jnode
	infoOpen map[string]bool

	// Settings page: the AID being typed, and whether the custom AID
	// field shows (also while the AID still equals a preset).
	customAID     string
	aidCustomMode bool
	aidInit       bool
	atPort        string

	// Log page.
	logList ui.ListState

	dl download
	// toast is a message for the next frame from a goroutine.
	toast string
	// clip is the system clipboard (replaced in tests).
	clip clipboard
	// appIcon is the decoded app icon of the About page.
	appIcon ui.ImageSource
	// demo is set when the card and servers are simulated.
	demo bool
	// The certificate shown, its tree, and which nodes are open.
	certOpen     bool
	certCI       *registry.Issuer
	certKeyID    string
	certTree     *jnode
	certTreeOpen map[string]bool
}

// clipboard is the part of the system clipboard the app uses.
type clipboard interface {
	ReadText() string
	ReadImage() []byte
	WriteText(s string)
}

func newGUI(clip clipboard) *gui {
	return &gui{page: pageProfiles, clip: clip, notifSort: ui.SortOrder{Column: "seq", Descending: true}}
}

// view builds the window from the state, every frame.
func (g *gui) view(c *ui.Context) {
	if g.app == nil {
		return
	}
	t := c.Theme()
	for _, n := range g.app.TakeNotices() {
		c.Toast(noticeText(n))
	}
	if g.toast != "" {
		c.Toast(g.toast)
		g.toast = ""
	}

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
	g.downloadModal(c)
	g.certificateDialog(c)
	g.profileDialogs(c)

	// Cmd/Ctrl+R reads the card again, Cmd/Ctrl+N adds an eSIM.
	if c.Shortcut(ui.Cmd, ui.KeyR) {
		g.refresh()
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && g.app.Loaded {
		g.openDownload()
	}
}

func (g *gui) refresh() {
	if g.app.CurrentDevice() == "" || g.app.Busy() {
		return
	}
	_ = g.app.Refresh()
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
	name := g.app.CurrentDevice()
	status, color := locales.DeviceNotConnected(), t.TextMuted
	switch {
	case name == "":
		name, status = locales.DeviceNone(), locales.DeviceChooseInSettings()
	case g.app.Busy():
		status, color = locales.DeviceWorking(), t.Accent
	case g.app.Loaded:
		status, color = locales.DeviceReady(), t.Success
	case !g.app.DevicePresent():
		status, color = locales.DeviceNotFound(), t.Warning
	case g.app.LastError != nil:
		status, color = locales.DeviceError(), t.Danger
	}
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
		g.page = pageSettings
	}
}

// pane is the page under its floating glass header.
func (g *gui) pane(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	top := float32(8) + headerHeight + 12
	bottom := float32(28)
	if g.app.Task != nil {
		bottom = 76 // the task pill floats there
	}
	ui.Scroll(c).Fill().Padding(top, 28, bottom).Gap(16).ScrollbarInsets(top, 0, 0).Children(func() {
		switch g.page {
		case pageNotifications:
			g.notificationsPage(c)
		case pageCard:
			g.cardPage(c)
		case pageSettings:
			g.settingsPage(c)
		case pageLog:
			g.logPage(c)
		case pageAbout:
			g.aboutPage(c)
		default:
			g.profilesPage(c)
		}
	})
	// Content scrolls under the header, faded by the scroll edge.
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(top + 8).PassThrough().Material(glass.ScrollEdge{})
	header := ui.Row(c).Absolute().Top(8).Left(12).Right(bar.Right+12).Height(headerHeight).
		Radius(headerHeight/2).Padding(0, 8, 0, 18).Gap(6).Material(glass.Glass{}).DragWindow()
	header.Children(func() {
		ui.Text(c, pageTitle(g.page)).FontSize(15).Bold().SingleLine().Grow(1).MinWidth(0)
		g.headerActions(c)
	})
	_ = t
	g.taskPill(c)
}

func pageTitle(p string) string {
	switch p {
	case pageNotifications:
		return locales.NavNotifications()
	case pageCard:
		return locales.NavCard()
	case pageSettings:
		return locales.NavSettings()
	case pageLog:
		return locales.NavLog()
	case pageAbout:
		return locales.NavAbout()
	}
	return locales.NavEsims()
}

func (g *gui) headerActions(c *ui.Context) {
	switch g.page {
	case pageProfiles, pageNotifications, pageCard:
		canRead := g.app.CurrentDevice() != "" && !g.app.Busy()
		if iconButton(c, iconRefresh, locales.ActionRefresh()).Disabled(!canRead).Clicked() {
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

// taskPill floats at the bottom while a card task runs.
func (g *gui) taskPill(c *ui.Context) {
	task := g.app.Task
	if task == nil || g.dl.open {
		return
	}
	t := c.Theme()
	ui.Row(c).Absolute().Left(0).Right(0).Bottom(20).Justify(ui.Center).PassThrough().Children(func() {
		ui.Row(c).Padding(8, 16).Gap(10).Radius(20).Material(glass.Glass{}).Children(func() {
			ui.Spinner(c).Size(16, 16)
			label := taskLabel(task.Kind)
			if task.Stage != "" {
				label = locales.TaskWithStage(label, stageLabel(task.Stage))
			}
			ui.Text(c, label).FontSize(13).SingleLine()
			_ = t
		})
	})
}

// iconButton is a button showing only an icon.
func iconButton(c *ui.Context, s *ui.SVG, label string) ui.Element {
	return ui.Button(c, "").Label(label).Tooltip(label).Radius(16).Children(func() {
		ui.Icon(c, s).Size(16, 16)
	})
}

// card is a rounded panel.
func card(c *ui.Context, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(16, 18).Gap(12).Radius(14).Background(t.Surface).
		Border(1, t.Border).Children(fn)
}

// sectionTitle heads a group of cards.
func sectionTitle(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(13).FontWeight(600).
		TextColor(c.Theme().TextMuted).Padding(8, 4, 0)
}

// emptyState is a centered message with an optional action.
func emptyState(c *ui.Context, s *ui.SVG, title, body string, action func()) {
	t := c.Theme()
	ui.Column(c).AlignSelf(ui.Stretch).Padding(64, 24).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(64, 64).Radius(20).Center().Material(glass.Glass{Tint: t.Accent.Alpha(0.25)}).Children(func() {
			ui.Icon(c, s).Size(30, 30).TextColor(t.Accent)
		})
		ui.Text(c, title).FontSize(17).Bold().Padding(8, 0, 0)
		if body != "" {
			ui.Text(c, body).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(420)
		}
		if action != nil {
			ui.Box(c).Height(6)
			action()
		}
	})
}

// identifier shows an EID or ICCID, obscured when the setting says so.
func (g *gui) identifier(s string) string {
	if g.app.Settings.HideIdentifiers {
		return app.Mask(s)
	}
	return s
}

// connectionHint explains why there is no card data.
func (g *gui) connectionHint(c *ui.Context) bool {
	if g.app.Loaded {
		return false
	}
	switch {
	case g.app.CurrentDevice() == "":
		title := locales.HintChooseReader()
		if g.app.Settings.Backend == settings.BackendAT {
			title = locales.HintChoosePort()
		}
		emptyState(c, deviceIcon(g.app.Settings.Backend), title, locales.HintChooseBody(), func() {
			if ui.PrimaryButton(c, locales.HintOpenSettings()).Clicked() {
				g.page = pageSettings
			}
		})
	case g.app.Busy():
		ui.Column(c).AlignSelf(ui.Stretch).Padding(80, 0).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, locales.HintReading()).TextColor(c.Theme().TextMuted)
		})
	case g.app.LastError != nil:
		emptyState(c, iconAlert, locales.HintReadFailed(), app.DescribeError(g.app.LastError), func() {
			if ui.PrimaryButton(c, locales.HintTryAgain()).Clicked() {
				g.refresh()
			}
		})
	case !g.app.DevicePresent():
		emptyState(c, deviceIcon(g.app.Settings.Backend), locales.HintDeviceGone(g.app.CurrentDevice()), locales.HintDeviceGoneBody(), func() {
			if ui.PrimaryButton(c, locales.HintOpenSettings()).Clicked() {
				g.page = pageSettings
			}
		})
	default:
		emptyState(c, iconSIM, locales.HintNotRead(), "", func() {
			if ui.PrimaryButton(c, locales.HintReadCard()).Clicked() {
				g.refresh()
			}
		})
	}
	return true
}

// taskLabel says what a task does.
func taskLabel(k app.TaskKind) string {
	switch k {
	case app.TaskRefresh:
		return locales.TaskRefresh()
	case app.TaskEnable:
		return locales.TaskEnable()
	case app.TaskDisable:
		return locales.TaskDisable()
	case app.TaskDelete:
		return locales.TaskDelete()
	case app.TaskRename:
		return locales.TaskRename()
	case app.TaskSetDefaultSMDP:
		return locales.TaskSetDefaultSmdp()
	case app.TaskSendNotifications:
		return locales.TaskSendNotifications()
	case app.TaskRemoveNotifications:
		return locales.TaskRemoveNotifications()
	case app.TaskDownload:
		return locales.TaskDownload()
	}
	return string(k)
}

// errorText explains an error, an SM-DP+ one by its codes' meaning.
func errorText(err error) string {
	if summary, codes, ok := app.ServerError(err); ok {
		return locales.NoticeServerError(summary, codes)
	}
	return err.Error()
}

// noticeText words the outcome of a task.
func noticeText(n app.Notice) string {
	switch n.Kind {
	case app.NoticeCancelled:
		return locales.NoticeCancelled(taskLabel(n.Task))
	case app.NoticeFailed:
		return locales.NoticeFailed(taskLabel(n.Task), errorText(n.Err))
	case app.NoticeSent:
		if n.Err != nil {
			return locales.NoticeSentPartly(n.Sent, n.Total, errorText(n.Err))
		}
		return locales.NoticeSent(n.Sent)
	case app.NoticeRemoved:
		return locales.NoticeRemoved(n.Total)
	}
	var done string
	switch n.Task {
	case app.TaskEnable:
		done = locales.NoticeEnabled()
	case app.TaskDisable:
		done = locales.NoticeDisabled()
	case app.TaskDelete:
		done = locales.NoticeDeleted()
	case app.TaskRename:
		return locales.NoticeRenamed()
	case app.TaskSetDefaultSMDP:
		return locales.NoticeDefaultSmdpSaved()
	default:
		return taskLabel(n.Task)
	}
	switch {
	case n.Unsent > 0:
		return locales.NoticeNotificationsUnsent(done, n.Unsent)
	case n.Sent > 0:
		return locales.NoticeWithNotifications(done, n.Sent)
	}
	return done
}

// copyButton is a small copy icon after a value; key tells apart the
// buttons of a list.
func copyButton(c *ui.Context, key, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("copy:" + key)).Padding(3).Radius(6).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, iconCopy).Size(13, 13).TextColor(t.TextMuted) })
	return b
}
