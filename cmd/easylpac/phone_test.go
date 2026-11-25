//go:build android || phone

package main

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/registry"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/locales"
)

// phoneFixture is a gui over a fake card behind a USB reader, a tester
// for each tab's window at a phone's size.
type phoneFixture struct {
	*fixture
	tabs  map[string]*ui.Tester
	got   settings.Backend // the reader's settings the service was made with
	reads atomic.Int32
	// fail, when set, is what reading the card fails with.
	fail atomic.Pointer[error]
}

// countingCard counts the reads of the card, and fails them on demand.
type countingCard struct {
	*fakeCard
	reads *atomic.Int32
	fail  *atomic.Pointer[error]
}

func (c countingCard) ChipInfo(ctx context.Context) (*lpa.EuiccInfo, error) {
	c.reads.Add(1)
	if err := c.fail.Load(); err != nil {
		return nil, *err
	}
	return c.fakeCard.ChipInfo(ctx)
}

const phoneReader = "/dev/bus/usb/001/002"

func newPhoneFixture(t *testing.T) *phoneFixture {
	t.Helper()
	card := newFakeCard()
	f := &phoneFixture{fixture: &fixture{g: newGUI(&memClipboard{}), q: make(chan func(), 256)}, tabs: map[string]*ui.Tester{}}
	cfg := settings.Default()
	cfg.Backend = settings.BackendCCID
	cfg.AutoNotify = true
	f.g.app = app.New(app.Options{
		Settings: cfg,
		Dispatch: func(fn func()) { f.q <- fn },
		Version:  "test",
		NewService: func(s *settings.Config, _ func(string)) (lpa.Service, error) {
			f.got = s.Backends[settings.BackendCCID]
			return countingCard{card, &f.reads, &f.fail}, nil
		},
		Discover: func(context.Context, string) ([]transport.Device, error) {
			return []transport.Device{{ID: phoneReader, Label: "ACS CCID USB Reader (072f:90cc)"}}, nil
		},
	})
	for _, tab := range phoneTabs {
		tt := ui.NewTester(f.g.pageView(tab.page), 360, 720)
		// Android's body text is 16, the Mac's 13: the phone's pages are
		// drawn here with the text of the phone.
		tt.SetPreferences(ui.Preferences{TextScale: 16.0 / 13})
		f.tabs[tab.page] = tt
	}
	f.tab(pageProfiles)
	return f
}

// tab chooses the tab of page, as the bar does, and draws it.
func (f *phoneFixture) tab(page string) {
	f.g.page = page
	f.tt = f.tabs[page]
	f.tt.Frame()
}

func TestPhoneTabs(t *testing.T) {
	f := newPhoneFixture(t)
	if f.g.chrome != frameChrome || frameChrome == desktopChrome {
		t.Fatalf("chrome %v, want the phone's", f.g.chrome)
	}

	// Before a reader is chosen the eSIM tab sends to the settings tab.
	f.click(t, "Open Settings")
	if f.g.page != pageSettings {
		t.Fatalf("page %q after Open Settings", f.g.page)
	}

	f.chooseReader(t, phoneReader)
	if !f.g.app.Loaded {
		t.Fatalf("card not read: %v", f.g.app.LastError)
	}
	if f.got.Reader != phoneReader || f.got.NoPPS {
		t.Fatalf("reader settings %+v", f.got)
	}

	f.tab(pageProfiles)
	for _, s := range []string{"Travel Data", "Work line"} {
		if !f.tt.HasText(s) {
			t.Fatalf("eSIM tab: missing %q; texts %q", s, f.tt.Texts())
		}
	}
	f.shot(t, "phone-profiles")
	f.click(t, "Add eSIM")
	if !f.g.dl.open {
		t.Fatal("the floating button did not open the download")
	}
	f.g.dl.open = false

	f.tab(pageNotifications)
	if !f.tt.HasText("rsp.example.com") {
		t.Fatalf("notifications tab; texts %q", f.tt.Texts())
	}
	f.shot(t, "phone-notifications")
	f.click(t, "Send all")
	f.settle(t)
	if len(f.g.app.Notifications) != 0 {
		t.Fatalf("%d notifications left after Send all", len(f.g.app.Notifications))
	}

	f.tab(pageCard)
	if !f.tt.HasText("lpa.ds.gsma.com") {
		t.Fatalf("card tab; texts %q", f.tt.Texts())
	}
	f.shot(t, "phone-card")

	f.tab(pageSettings)
	f.shot(t, "phone-settings")
	f.tt.Scroll(180, 360, 0, 5000)
	f.tt.Frame()
	f.shot(t, "phone-settings-end")
	f.click(t, "Log")
	if !f.g.front(pageLog) || f.g.front(pageSettings) {
		t.Fatalf("pushed %q: the log should be in front", f.g.phone.pushed)
	}
}

// Only the window in front shows what tasks leave to say, once.
func TestPhoneNoticesInFront(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageNotifications)
	f.g.toast = "hello"
	f.tabs[pageProfiles].Frame()
	if f.g.toast != "hello" {
		t.Fatal("a tab behind took the toast")
	}
	f.tt.Frame()
	if f.g.toast != "" {
		t.Fatal("the tab in front did not take the toast")
	}
}

// A pull reads the card again, and its indicator goes once the read is
// over; with nothing to read it goes at once. The gesture itself needs a
// phone: desktops have no overscroll to pull.
func TestPhonePulled(t *testing.T) {
	f := newPhoneFixture(t)
	pull := f.g.pullState(pageCard)
	pull.Refreshing = true
	f.g.pulled(pull)
	if pull.Refreshing {
		t.Fatal("refreshing with no reader chosen")
	}

	f.chooseReader(t, phoneReader)
	f.tab(pageCard)
	before := f.reads.Load()
	pull.Refreshing = true
	f.g.pulled(pull)
	if !f.g.app.Busy() {
		t.Fatal("the pull did not start a read")
	}
	f.tt.Frame()
	if !pull.Refreshing {
		t.Fatal("the indicator went while reading")
	}
	f.settle(t)
	if f.reads.Load() == before {
		t.Fatal("the card was not read")
	}
	if pull.Refreshing {
		t.Fatal("still refreshing after the read")
	}
}

// The notifications are a list a tap checks, without the desktop's hint
// of modifier keys, and the checked ones send on their own.
func TestPhoneNotificationList(t *testing.T) {
	f := newPhoneFixture(t)
	f.g.app.UpdateSettings(func(s *settings.Config) { s.AutoNotify = false })
	f.chooseReader(t, phoneReader)
	card := f.g.app.Notifications
	f.tab(pageNotifications)
	if f.tt.HasText("Shift-click") {
		t.Fatal("the hint of modifier keys shows on a phone")
	}
	f.shot(t, "phone-notification-list")
	f.click(t, "#1")
	if !f.g.notifSel.Has(1) {
		t.Fatal("a tap on the row did not check it")
	}
	f.tt.Frame()
	f.click(t, "Choose notification 1")
	if f.g.notifSel.Has(1) {
		t.Fatal("the checkbox did not uncheck it")
	}
	f.tt.Frame()
	f.click(t, "#1")
	f.tt.Frame()
	f.click(t, "Send 1 selected")
	f.settle(t)
	if len(f.g.app.Notifications) != len(card)-1 {
		t.Fatalf("%d notifications left, want %d", len(f.g.app.Notifications), len(card)-1)
	}
}

// Settings choose a reader by its name, and set PPS, with no choice of
// PC/SC or modem.
func TestPhoneSettings(t *testing.T) {
	f := newPhoneFixture(t)
	f.g.app.RefreshDevices()
	f.until(t, func() bool { return !f.g.app.Discovering })
	f.tab(pageSettings)
	if f.tt.HasText("Connect through") || f.tt.HasText("Modem (AT)") {
		t.Fatal("the choice of backend shows on a phone")
	}
	f.g.app.SelectDevice(phoneReader)
	f.until(t, func() bool { return f.g.app.Loaded })
	f.tab(pageSettings)
	if !f.tt.HasText("ACS CCID USB Reader (072f:90cc)") || f.tt.HasText(phoneReader) {
		t.Fatalf("the reader is not shown by its name; texts %q", f.tt.Texts())
	}
	f.shot(t, "phone-settings-reader")
	sw, ok := f.tt.AccessByRole(ui.RoleSwitch, "Faster card rate")
	if !ok {
		t.Fatal("no PPS switch")
	}
	f.tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if !f.g.app.Settings.Backends[settings.BackendCCID].NoPPS {
		t.Fatal("PPS still on")
	}

	// A custom AID's 32 digits get the width of the page, and the button
	// to apply it goes below them.
	f.g.aidCustomMode = true
	f.g.customAID = "A0000005591010FFFFFFFF8900000177"
	f.tab(pageSettings)
	f.shot(t, "phone-settings-aid")
	in, ok := f.tt.AccessByRole(ui.RoleTextField, "Custom AID")
	if !ok {
		t.Fatalf("no AID field; texts %q", f.tt.Texts())
	}
	apply, ok := f.tt.AccessByRole(ui.RoleButton, "Apply")
	if !ok {
		t.Fatal("no Apply button")
	}
	if in.Bounds.W < 260 || apply.Bounds.Y < in.Bounds.Y+in.Bounds.H {
		t.Fatalf("AID field %+v, Apply %+v: want the field the page's width, the button below it", in.Bounds, apply.Bounds)
	}
}

// Dialogs fit the window.
func TestPhoneDialogsFit(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageCard)
	f.click(t, "Change…")
	f.tt.Frame()
	f.shot(t, "phone-smdp-dialog")
	for _, s := range []string{"Cancel", "Save"} {
		r, ok := f.tt.Find(s)
		if !ok || r.X < 0 || r.X+r.W > 360 {
			t.Fatalf("%q at %+v, outside the window", s, r)
		}
	}
	f.g.editSMDP = false
	f.tab(pageProfiles)
	f.click(t, "Add eSIM")
	f.tt.Frame()
	f.shot(t, "phone-download")
	// Taller than the window (with the demo's codes), it scrolls down to
	// its buttons.
	f.tt.Scroll(180, 300, 0, 2000)
	f.tt.Frame()
	f.shot(t, "phone-download-end")
	r, ok := f.tt.Find("Cancel")
	if !ok || r.X < 0 || r.X+r.W > 360 || r.Y < 0 || r.Y+r.H > 720 {
		t.Fatalf("Cancel at %+v, outside the window; texts %q", r, f.tt.Texts())
	}

	f.click(t, "Advanced")
	f.tt.Advance(time.Second) // the section opens
	f.tt.Frame()
	f.tt.Scroll(180, 300, 0, 2000)
	f.tt.Frame()
	f.shot(t, "phone-download-advanced")
}

// Deleting the eSIM in use says why it can't, as phones show no tooltip.
func TestPhoneDeleteInUse(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	f.click(t, "More for Travel Data")
	f.tt.Frame()
	if err := f.tt.ChooseMenuItem("Delete…"); err != nil {
		t.Fatalf("%v; menu %q", err, f.tt.Menu())
	}
	f.tt.Frame()
	if f.g.deleting {
		t.Fatal("the delete dialog opened for the eSIM in use")
	}
	if !f.tt.HasText(locales.ProfileDeleteDisabledHint()) {
		t.Fatalf("no reason shown; texts %q", f.tt.Texts())
	}
}

// A reader unplugged is told in words, not by usbfs's error.
func TestPhoneReaderGone(t *testing.T) {
	f := newPhoneFixture(t)
	gone := transport.Kind(transport.ErrDeviceGone, errors.New("usbfs: device disconnected"))
	f.fail.Store(&gone)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	if !f.tt.HasText(locales.ErrorDeviceGone()) || f.tt.HasText("usbfs") {
		t.Fatalf("texts %q", f.tt.Texts())
	}
	f.shot(t, "phone-reader-gone")
}

// Phones read a QR code from the camera or a photo, and the photo, which
// holds the activation code, goes once read.
func TestPhoneQRPhoto(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	f.click(t, "Add eSIM")
	f.tt.Frame()
	for _, s := range []string{"Scan QR code", "Choose photo"} {
		if !f.tt.HasText(s) {
			t.Fatalf("no %q; texts %q", s, f.tt.Texts())
		}
	}
	if f.tt.HasText("Open QR image…") || f.tt.HasText("drop") {
		t.Fatal("the desktop's ways show on a phone")
	}

	const code = "LPA:1$smdp.example.com$MATCH-1"
	m, err := qrcode.NewQRCodeWriter().Encode(code, gozxing.BarcodeFormat_QR_CODE, 300, 300, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(out, m); err != nil {
		t.Fatal(err)
	}
	out.Close()
	f.g.foundQR(readQRPhoto(path, nil))
	if f.g.dl.code != code {
		t.Fatalf("code %q", f.g.dl.code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the photo was kept")
	}
}

// A photo without a QR code says so, and goes too.
func TestPhoneQRPhotoWithoutCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "photo.png")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(out, image.NewGray(image.Rect(0, 0, 64, 64)))
	out.Close()
	g := newGUI(&memClipboard{})
	g.foundQR(readQRPhoto(path, nil))
	// In a toast: the form's end is out of sight on a phone.
	if g.toast != locales.DlNoQrImage() || g.dl.inputError != "" {
		t.Fatalf("toast %q, error %q", g.toast, g.dl.inputError)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the photo was kept")
	}
}

// The fields typed in are one line high, after the code's taller field.
func TestPhoneManualFields(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	f.click(t, "Add eSIM")
	f.tt.Frame()
	f.click(t, "Enter details manually")
	f.tt.Frame()
	f.tt.Frame()
	f.shot(t, "phone-download-manual")
	smdp, ok := f.tt.AccessByRole(ui.RoleTextField, "SM-DP+ address")
	if !ok {
		t.Fatalf("no SM-DP+ field; texts %q", f.tt.Texts())
	}
	id, ok := f.tt.AccessByRole(ui.RoleTextField, "Matching ID")
	if !ok {
		t.Fatal("no matching ID field")
	}
	if smdp.Bounds.H > id.Bounds.H+1 {
		t.Fatalf("SM-DP+ field %v high, matching ID %v", smdp.Bounds.H, id.Bounds.H)
	}
	// And back: the code's field is taller again, for its lines.
	f.click(t, "Use an activation code")
	f.tt.Frame()
	f.tt.Frame()
	code, ok := f.tt.AccessByRole(ui.RoleTextField, "Activation code")
	if !ok {
		t.Fatalf("no activation code field; texts %q", f.tt.Texts())
	}
	if code.Bounds.H < 1.5*id.Bounds.H {
		t.Fatalf("activation code field %v high, want two lines at least", code.Bounds.H)
	}
}

// The About page fits a phone, a snapshot's long version too, and the
// names of a download's functions aren't broken inside a word.
func TestPhoneAboutAndStages(t *testing.T) {
	old := Version
	Version = "0.9.3.r20.ge812fbc.dirty"
	t.Cleanup(func() { Version = old })
	f := newPhoneFixture(t)
	about := ui.NewTester(f.g.pageView(pageAbout), 360, 720)
	about.SetPreferences(ui.Preferences{TextScale: 16.0 / 13})
	f.tabs[pageAbout] = about
	f.tab(pageAbout)
	f.shot(t, "phone-about")
	r, ok := f.tt.Find(locales.AboutVersion(Version))
	if !ok || r.X+r.W > 360-phonePadding {
		t.Fatalf("version at %+v, past the card; texts %q", r, f.tt.Texts())
	}
	date, ok1 := f.tt.Find(registry.DataVersion)
	mygoName, ok2 := f.tt.Find("MyGo")
	if !ok1 || !ok2 || date.H > 1.5*mygoName.H {
		t.Fatalf("data version %+v, a line is %v high: want it on one line", date, mygoName.H)
	}

	f.tab(pageProfiles)
	at := time.Now()
	var marks []app.StageMark
	for i, st := range downloadStages[:2] {
		marks = append(marks, app.StageMark{Name: st.id, At: at.Add(time.Duration(i-2) * 450 * time.Millisecond)})
	}
	f.g.dl = download{open: true, step: dlRunning}
	f.g.app.Task = &app.Task{Kind: app.TaskDownload, Stage: marks[1].Name, Stages: marks}
	f.tt.Frame()
	f.shot(t, "phone-download-stages")
	// Only the stage under way shows, not the list of them all.
	if !f.tt.HasText(locales.StageContactServer()) {
		t.Fatalf("the stage under way doesn't show; texts %q", f.tt.Texts())
	}
	for _, other := range []string{locales.StagePrepareCard(), locales.StageCheckServer(), locales.StageInstall()} {
		if f.tt.HasText(other) {
			t.Fatalf("%q shows besides the stage under way; texts %q", other, f.tt.Texts())
		}
	}
	long, ok1 := f.tt.Find("ES9+.InitiateAuthentication")
	label, ok2 := f.tt.Find(locales.StageContactServer())
	if !ok1 || !ok2 || long.H > label.H {
		t.Fatalf("function name %+v, the stage's name %+v: want it on one line; texts %q", long, label, f.tt.Texts())
	}

	// Cancelling, the cancel stage shows instead.
	marks = append(marks, app.StageMark{Name: cancelStage.id, At: at})
	f.g.app.Task = &app.Task{Kind: app.TaskDownload, Stage: cancelStage.id, Stages: marks}
	f.tt.Frame()
	if !f.tt.HasText(locales.StageCancel()) || f.tt.HasText(locales.StageContactServer()) {
		t.Fatalf("cancelling shows %q", f.tt.Texts())
	}
	f.g.app.Task = nil
}

// Sideways, with the navigation bar at the right, the add button keeps
// clear of it as the page does.
func TestPhoneAddButtonClearOfBars(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	f.tt.SetSize(720, 360)
	f.tt.SetInsets(0, 48, 0, 0)
	f.tt.Frame()
	f.shot(t, "phone-profiles-sideways")
	b, ok := f.tt.AccessByRole(ui.RoleButton, "Add eSIM")
	if !ok {
		t.Fatalf("no add button; texts %q", f.tt.Texts())
	}
	if b.Bounds.X+b.Bounds.W > 720-48 {
		t.Fatalf("add button at %+v, under the navigation bar at x 672", b.Bounds)
	}
}

// With Log pushed over the settings tab, a dialog opened on the eSIM tab
// shows there, not in the hidden Log page.
func TestPhoneDialogWithPagePushed(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.g.push(pageLog)
	log := ui.NewTester(f.g.pageView(pageLog), 360, 720)
	f.tab(pageProfiles)
	if !f.g.front(pageProfiles) || f.g.front(pageLog) {
		t.Fatal("the Log page pushed over Settings is in front on the eSIM tab")
	}
	f.g.renameICCID, f.g.renaming = "8985200012345678901", true
	f.tt.Frame()
	log.Frame()
	if !f.tt.HasText("Rename eSIM") {
		t.Fatalf("no rename dialog on the eSIM tab; texts %q", f.tt.Texts())
	}
	if log.HasText("Rename eSIM") {
		t.Fatal("the rename dialog shows in the Log page")
	}
	// On the settings tab the pushed page is in front again.
	f.g.renaming = false
	f.tab(pageSettings)
	if !f.g.front(pageLog) || f.g.front(pageSettings) {
		t.Fatal("on the settings tab, Log isn't in front")
	}
}

// While a dialog is open, choosing another tab goes back to the dialog's.
func TestPhoneDialogKeepsItsTab(t *testing.T) {
	f := newPhoneFixture(t)
	f.chooseReader(t, phoneReader)
	f.tab(pageProfiles)
	f.click(t, "Add eSIM")
	f.tt.Frame()
	if back := f.g.tabChosen(pageCard); back != pageProfiles || f.g.page != pageProfiles {
		t.Fatalf("chose Card with a dialog open: back %q, page %q", back, f.g.page)
	}
	f.g.dl.open = false
	f.tt.Frame()
	if back := f.g.tabChosen(pageCard); back != "" || f.g.page != pageCard {
		t.Fatalf("chose Card with no dialog: back %q, page %q", back, f.g.page)
	}
}
