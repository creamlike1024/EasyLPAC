package main

import (
	"errors"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/registry"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/locales"
)

// titleBarHeight is the height of a desktop window's title bar.
const titleBarHeight = 52

// The pages, by their ids.
const (
	pageProfiles      = "profiles"
	pageNotifications = "notifications"
	pageCard          = "card"
	pageSettings      = "settings"
	pageLog           = "log"
	pageAbout         = "about"
	// pageCertificate is a CI's certificate, on phones: desktops show it
	// in a dialog.
	pageCertificate = "certificate"
)

// gui is the state of the window: what the app layer does not own.
type gui struct {
	// win is the window dialogs of the system belong to; wins are all the
	// windows showing the state, which update redraws.
	win  *mygo.Window
	wins []*mygo.Window
	// lead is a window on screen, which update runs its changes through:
	// goroutines read it, the main thread sets it.
	lead atomic.Pointer[mygo.Window]
	app  *app.App
	// page is the page shown: the sidebar's choice, or the tab chosen.
	page string
	// selectTab, set on phones, chooses the tab of a page.
	selectTab func(page string)
	// phone is the phone frame's state.
	phone phoneState
	// chrome is the height the frame takes from a page: the header and the
	// paddings above and below it.
	chrome float32

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
	// locale is the language chosen, "" to follow the user's.
	locale string

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
	return &gui{page: pageProfiles, chrome: frameChrome, clip: clip, notifSort: ui.SortOrder{Column: "seq", Descending: true}}
}

// update runs fn on the main thread, then redraws the windows on screen:
// it is the app's Dispatch, and how goroutines change the state.
func (g *gui) update(fn func()) {
	w := g.lead.Load()
	if w == nil {
		return
	}
	w.Update(func() {
		fn()
		for _, x := range g.shownWins() {
			if x != w {
				x.Invalidate()
			}
		}
	})
}

// invalidate redraws the windows on screen, on the main thread. A hidden
// one draws when it shows: on phones, drawing the hidden tabs' windows
// too made every change several frames that the screen waits on.
func (g *gui) invalidate() {
	shown := g.shownWins()
	if len(shown) > 0 {
		g.lead.Store(shown[0])
	}
	for _, w := range shown {
		w.Invalidate()
	}
}

// show goes to a page: the sidebar's item, or on phones its tab.
func (g *gui) show(page string) {
	g.page = page
	if g.selectTab != nil {
		g.selectTab(page)
	}
}

// canRefresh reports whether the card can be read now.
func (g *gui) canRefresh() bool {
	return g.app.CurrentDevice() != "" && !g.app.Busy()
}

func (g *gui) refresh() {
	if !g.canRefresh() {
		return
	}
	_ = g.app.Refresh()
}

// showNotices shows what tasks and goroutines left to say, as toasts.
func (g *gui) showNotices(c *ui.Context) {
	for _, n := range g.app.TakeNotices() {
		c.Toast(noticeText(n))
	}
	if g.toast != "" {
		c.Toast(g.toast)
		g.toast = ""
	}
}

// dialogs builds the dialogs that any page may have open.
func (g *gui) dialogs(c *ui.Context) {
	g.downloadModal(c)
	g.certificateDialog(c)
	g.profileDialogs(c)
}

// pageBody builds what a page holds, without the frame around it.
func (g *gui) pageBody(c *ui.Context, page string) {
	switch page {
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
	case pageCertificate:
		g.certificatePage(c)
	default:
		g.profilesPage(c)
	}
}

// deviceState names the device in use and says how it is.
func (g *gui) deviceState(c *ui.Context) (name, status string, color ui.Color) {
	t := c.Theme()
	name = g.app.CurrentDevice()
	status, color = locales.DeviceNotConnected(), t.TextMuted
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
	return name, status, color
}

// taskText says what a running task does, and its stage.
func taskText(task *app.Task) string {
	label := taskLabel(task.Kind)
	if task.Stage != "" {
		label = locales.TaskWithStage(label, stageLabel(task.Stage))
	}
	return label
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
	case pageCertificate:
		return locales.CardViewCertificate()
	}
	return locales.NavEsims()
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
				g.show(pageSettings)
			}
		})
	case g.app.Busy():
		ui.Column(c).AlignSelf(ui.Stretch).Padding(80, 0).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, locales.HintReading()).TextColor(c.Theme().TextMuted)
		})
	case g.app.LastError != nil:
		emptyState(c, iconAlert, locales.HintReadFailed(), readErrorText(g.app.LastError), func() {
			if ui.PrimaryButton(c, locales.HintTryAgain()).Clicked() {
				g.refresh()
			}
		})
	case !g.app.DevicePresent():
		emptyState(c, deviceIcon(g.app.Settings.Backend), locales.HintDeviceGone(g.deviceLabel(g.app.CurrentDevice())), locales.HintDeviceGoneBody(), func() {
			if ui.PrimaryButton(c, locales.HintOpenSettings()).Clicked() {
				g.show(pageSettings)
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
	if s := deviceErrorText(err); s != "" {
		return s
	}
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

// taskPill floats at the bottom while a card task runs.
func (g *gui) taskPill(c *ui.Context) {
	task := g.app.Task
	if task == nil || g.dl.open {
		return
	}
	// Clear of what covers a phone's edges; nothing does on desktops.
	_, sr, sb, sl := c.SafeArea()
	ui.Row(c).Absolute().Left(sl).Right(sr).Bottom(20 + sb).Justify(ui.Center).PassThrough().Children(func() {
		ui.Row(c).Padding(8, 16).Gap(10).Radius(20).Material(glass.Glass{}).Children(func() {
			ui.Spinner(c).Size(16, 16)
			ui.Text(c, taskText(task)).FontSize(13).SingleLine()
		})
	})
}

// dialogWidth is the width of a dialog's content: w, or on phones what
// the window leaves of it beside the panel's margins and padding (the
// panel stays 20 from each edge, and pads 20 within).
func dialogWidth(c *ui.Context, w float32) float32 {
	if !phoneUI {
		return w
	}
	ww, _ := c.Size()
	return max(min(w, ww-80), 0)
}

// autoFocus focuses e as it shows, on desktops: on phones that would
// bring the keyboard up over a dialog before the user chose to type.
func autoFocus(e ui.Element) ui.Element {
	if !phoneUI {
		e.AutoFocus()
	}
	return e
}

// deviceErrorText explains what went wrong with the reader or the card,
// or is "" for other errors.
func deviceErrorText(err error) string {
	switch {
	case errors.Is(err, transport.ErrAccessDenied):
		return locales.ErrorAccessDenied()
	case errors.Is(err, transport.ErrDeviceGone):
		return locales.ErrorDeviceGone()
	case errors.Is(err, transport.ErrNoCard):
		return locales.ErrorNoCard()
	case errors.Is(err, transport.ErrCardMute):
		return locales.ErrorCardMute()
	}
	return ""
}

// readErrorText explains why the card could not be read.
func readErrorText(err error) string {
	if s := deviceErrorText(err); s != "" {
		return s
	}
	return app.DescribeError(err)
}
