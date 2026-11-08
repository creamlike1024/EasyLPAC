package main

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/locales"
)

// fakeCard is an in-memory eUICC for view tests.
type fakeCard struct {
	mu            sync.Mutex
	profiles      []*lpa.Profile
	notifications []*lpa.Notification
	seq           int
}

func nick(s string) *string { return &s }

func newFakeCard() *fakeCard {
	f := &fakeCard{profiles: []*lpa.Profile{
		{Iccid: "8985200012345678901", ProfileState: "enabled", ProfileName: "Travel Data", ServiceProviderName: "Example Mobile", ProfileClass: "operational"},
		{Iccid: "8944100098765432109", ProfileState: "disabled", ProfileName: "Home", ProfileNickname: nick("Work line"), ServiceProviderName: "Carrier Two", ProfileClass: "operational"},
	}}
	f.add("install", "8944100098765432109")
	return f
}

func (f *fakeCard) add(op, iccid string) {
	f.seq++
	f.notifications = append(f.notifications, &lpa.Notification{SeqNumber: f.seq, ProfileManagementOperation: op, Iccid: iccid, NotificationAddress: "rsp.example.com"})
}

func (f *fakeCard) Backends(context.Context) ([]string, error)            { return nil, nil }
func (f *fakeCard) Devices(context.Context, string) ([]lpa.Device, error) { return nil, nil }
func (f *fakeCard) Version(context.Context) (string, error)               { return "", nil }
func (f *fakeCard) ChipDefaultSmdp(context.Context, string) error         { return nil }
func (f *fakeCard) ProfileNickname(_ context.Context, iccid, n string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		if p.Iccid == iccid {
			p.ProfileNickname = nick(n)
		}
	}
	return nil
}
func (f *fakeCard) ChipInfo(context.Context) (*lpa.EuiccInfo, error) {
	info := &lpa.EuiccInfo{EidValue: "89049032000001000000123456789068"}
	info.EuiccConfiguredAddresses.RootDsAddress = "lpa.ds.gsma.com"
	info.EUICCInfo2.Svn = "2.2.2"
	info.EUICCInfo2.ProfileVersion = "2.1.0"
	info.EUICCInfo2.EuiccFirmwareVer = "4.2.0"
	info.EUICCInfo2.ExtCardResource.FreeNonVolatileMemory = 401203
	info.EUICCInfo2.ExtCardResource.InstalledApplication = 2
	info.EUICCInfo2.EuiccCiPKIDListForVerification = []string{"81370f5125d0b1d408d4c3b232e6d25e795bebfb"}
	info.EUICCInfo2.RspCapability = []string{"additionalProfile", "testProfileSupport"}
	info.EUICCInfo2.SasAcreditationNumber = "HG-BG-UP-0926"
	return info, nil
}
func (f *fakeCard) ProfileList(context.Context) ([]*lpa.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*lpa.Profile, len(f.profiles))
	for i, p := range f.profiles {
		cp := *p
		out[i] = &cp
	}
	return out, nil
}
func (f *fakeCard) NotificationList(context.Context) ([]*lpa.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*lpa.Notification(nil), f.notifications...), nil
}
func (f *fakeCard) setOnly(iccid string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		switch {
		case p.Iccid == iccid && on:
			p.ProfileState = "enabled"
		case p.Iccid == iccid || on:
			p.ProfileState = "disabled"
		}
	}
	op := "disable"
	if on {
		op = "enable"
	}
	f.add(op, iccid)
}
func (f *fakeCard) ProfileEnable(_ context.Context, iccid string) error {
	f.setOnly(iccid, true)
	return nil
}
func (f *fakeCard) ProfileDisable(_ context.Context, iccid string) error {
	f.setOnly(iccid, false)
	return nil
}
func (f *fakeCard) ProfileDelete(context.Context, string) error { return nil }
func (f *fakeCard) NotificationProcess(_ context.Context, seq int, remove bool) error {
	if remove {
		return f.NotificationRemove(context.Background(), seq)
	}
	return nil
}
func (f *fakeCard) NotificationRemove(_ context.Context, seq int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, n := range f.notifications {
		if n.SeqNumber == seq {
			f.notifications = append(f.notifications[:i], f.notifications[i+1:]...)
			break
		}
	}
	return nil
}
func (f *fakeCard) ProfileDownload(_ context.Context, req lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	meta := lpa.ProfileMetadata{ICCID: "8981100055555555555", ProfileName: "New Plan", ServiceProviderName: "Example Mobile"}
	ok, err := req.Confirm(meta)
	if err != nil || !ok {
		return nil, &lpa.OpError{Stage: lpa.StageDownload, Op: "preview", Cause: lpa.ErrUserCancelled}
	}
	f.mu.Lock()
	f.profiles = append(f.profiles, &lpa.Profile{Iccid: meta.ICCID, ProfileState: "disabled", ProfileName: meta.ProfileName, ServiceProviderName: meta.ServiceProviderName})
	f.add("install", meta.ICCID)
	n := f.notifications[len(f.notifications)-1]
	f.mu.Unlock()
	return &lpa.DownloadOutcome{Notification: n}, nil
}

// memClipboard stands in for the system clipboard.
type memClipboard struct {
	text string
	img  []byte
}

func (m *memClipboard) ReadText() string   { return m.text }
func (m *memClipboard) ReadImage() []byte  { return m.img }
func (m *memClipboard) WriteText(s string) { m.text = s }

// fixture is a gui over a fake card, its dispatches run by settle.
type fixture struct {
	g  *gui
	tt *ui.Tester
	q  chan func()
}

func newFixture(t *testing.T, card *fakeCard) *fixture {
	t.Helper()
	f := &fixture{g: newGUI(&memClipboard{}), q: make(chan func(), 256)}
	cfg := settings.Default()
	cfg.AutoNotify = true
	f.g.app = app.New(app.Options{
		Settings: cfg,
		Dispatch: func(fn func()) { f.q <- fn },
		Version:  "test",
		NewService: func(*settings.Config, func(string)) (lpa.Service, error) {
			return card, nil
		},
		Discover: func(context.Context, string) ([]transport.Device, error) {
			return []transport.Device{{ID: "ACS ACR38U-CCID", Label: "ACS ACR38U-CCID"}}, nil
		},
	})
	f.tt = ui.NewTester(f.g.view, 1040, 700)
	f.tt.SetTitleBar(ui.TitleBar{Height: titleBarHeight, Left: 76})
	return f
}

// settle runs dispatched work until the app is idle (or waits on a
// preview) and nothing has been dispatched for a moment, then draws.
func (f *fixture) settle(t *testing.T) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case fn := <-f.q:
			fn()
		case <-time.After(30 * time.Millisecond):
			if !f.g.app.Busy() || f.g.app.Confirm != nil {
				f.tt.Frame()
				return
			}
		case <-deadline:
			t.Fatal("app did not settle")
		}
	}
}

// until runs dispatched work until cond holds, then draws.
func (f *fixture) until(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for !cond() {
		select {
		case fn := <-f.q:
			fn()
		case <-deadline:
			t.Fatal("condition not reached")
		}
	}
	f.tt.Frame()
}

// chooseReader does what the user does after launch: the devices found
// at launch show in Settings, where choosing one reads its card.
func (f *fixture) chooseReader(t *testing.T, reader string) {
	t.Helper()
	f.g.app.RefreshDevices()
	f.until(t, func() bool { return !f.g.app.Discovering })
	f.g.app.SelectDevice(reader)
	f.until(t, func() bool { return f.g.app.Loaded || f.g.app.LastError != nil })
}

func (f *fixture) click(t *testing.T, label string) {
	t.Helper()
	if err := f.tt.Click(label); err != nil {
		t.Fatalf("click %q: %v; texts %q", label, err, f.tt.Texts())
	}
}

// shot writes the frame to $EASYLPAC_SHOTS/<name>.png when set.
func (f *fixture) shot(t *testing.T, name string) {
	dir := os.Getenv("EASYLPAC_SHOTS")
	if dir == "" {
		return
	}
	out, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, f.tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestPagesAndProfileFlow(t *testing.T) {
	card := newFakeCard()
	f := newFixture(t, card)
	f.tt.Frame()
	f.chooseReader(t, "ACS ACR38U-CCID")
	if !f.g.app.Loaded {
		t.Fatalf("the reader was not read on discovery; texts %q", f.tt.Texts())
	}
	for _, s := range []string{"Travel Data", "Work line", "In use", "Other eSIMs"} {
		if !f.tt.HasText(s) {
			t.Fatalf("missing %q; texts %q", s, f.tt.Texts())
		}
	}
	f.shot(t, "profiles")

	// Turning the second eSIM on goes through the app and sends its
	// notification, leaving the older one pending.
	if err := f.tt.Click("Use Work line"); err != nil {
		t.Fatal(err)
	}
	f.settle(t)
	for _, p := range card.profiles {
		if p.Iccid == "8944100098765432109" && p.ProfileState != "enabled" {
			t.Fatal("profile not enabled")
		}
	}
	if len(card.notifications) != 1 {
		t.Fatalf("notifications left %d, want the older install one", len(card.notifications))
	}

	f.click(t, "Notifications")
	f.tt.Frame()
	if !f.tt.HasText("Installed") || !f.tt.HasText("rsp.example.com") {
		t.Fatalf("notification table; texts %q", f.tt.Texts())
	}
	f.shot(t, "notifications")
	f.click(t, "Send all")
	f.settle(t)
	if len(card.notifications) != 0 || !f.tt.HasText("No pending notifications") {
		t.Fatalf("after send all: %d left; texts %q", len(card.notifications), f.tt.Texts())
	}

	f.click(t, "Card")
	f.tt.Frame()
	if !f.tt.HasText("lpa.ds.gsma.com") || !f.tt.HasText("2.2.2") || !f.tt.HasText("89049032000001000000123456789068") {
		t.Fatalf("card page; texts %q", f.tt.Texts())
	}
	if f.tt.HasText("The check digits of this EID do not verify (SGP.29).") {
		t.Fatal("valid EID reported invalid")
	}
	f.click(t, "Expand all")
	f.tt.Frame()
	if !f.tt.HasText("GSM Association - RSP2 Root CI1") || !f.tt.HasText("BHDC · 🇨🇳 CN") {
		t.Fatalf("EUICCInfo2 notes missing; texts %q", f.tt.Texts())
	}
	if !f.tt.HasText("🇩🇪 G+D") || !f.tt.HasText("🇨🇳 BHDC") || !f.tt.HasText("HG-BG-UP-0926") || !f.tt.HasText("2 eSIMs installed") {
		t.Fatalf("EUM manufacturer missing; texts %q", f.tt.Texts())
	}
	f.click(t, "Copy JSON")
	if clip := f.g.clip.(*memClipboard).text; !strings.Contains(clip, `"sasAcreditationNumber": "HG-BG-UP-0926"`) {
		t.Fatalf("clipboard %q", clip)
	}
	f.shot(t, "card")

	f.click(t, "Settings")
	f.tt.Frame()
	if !f.tt.HasText("Card reader (PC/SC)") || !f.tt.HasText("Standard (GSMA)") {
		t.Fatalf("settings page; texts %q", f.tt.Texts())
	}
	f.shot(t, "settings")

	f.click(t, "Log")
	f.tt.Frame()
	if !f.tt.HasText("Export…") || !f.tt.HasText("EIDs and ICCIDs show in full. To copy or share this log, turn on Obscure identifiers in Settings first.") {
		t.Fatalf("log page; texts %q", f.tt.Texts())
	}
	f.shot(t, "log")
}

func TestDownloadFlow(t *testing.T) {
	card := newFakeCard()
	f := newFixture(t, card)
	f.tt.Frame()
	f.chooseReader(t, "ACS ACR38U-CCID")

	f.click(t, "Add eSIM")
	f.tt.Frame()
	f.g.dl.code = "LPA:1$rsp.example.com$ABCD-1234"
	f.tt.Frame()
	if !f.tt.HasText("rsp.example.com") {
		t.Fatalf("parsed code not shown; texts %q", f.tt.Texts())
	}
	f.shot(t, "download-input")
	f.click(t, "Download")
	f.settle(t)
	if f.g.app.Confirm == nil || !f.tt.HasText("Install this eSIM?") {
		t.Fatalf("no preview; texts %q", f.tt.Texts())
	}
	f.shot(t, "download-preview")
	f.click(t, "Install")
	f.settle(t)
	if f.g.dl.step != dlDone || !f.tt.HasText("eSIM added") {
		t.Fatalf("step %d err %v; texts %q", f.g.dl.step, f.g.dl.err, f.tt.Texts())
	}
	f.shot(t, "download-done")
	if len(card.profiles) != 3 || len(card.notifications) != 1 {
		t.Fatalf("profiles %d notifications %d", len(card.profiles), len(card.notifications))
	}
	f.click(t, "Done")
	f.tt.Frame()
	if !f.tt.HasText("New Plan") {
		t.Fatalf("new profile not listed; texts %q", f.tt.Texts())
	}
}

// A launch chooses nothing: every card page asks for a device, Settings
// lists what the one look found, and choosing a reader reads it.
func TestLaunchChoosesNoDevice(t *testing.T) {
	f := newFixture(t, newFakeCard())
	f.g.app.RefreshDevices()
	f.until(t, func() bool { return !f.g.app.Discovering })
	if f.g.app.CurrentDevice() != "" || f.g.app.Loaded || f.g.app.Busy() {
		t.Fatalf("launch chose %q, loaded %v", f.g.app.CurrentDevice(), f.g.app.Loaded)
	}
	for _, page := range []string{pageProfiles, pageNotifications, pageCard} {
		f.g.page = page
		f.tt.Frame()
		if !f.tt.HasText("Choose a card reader") || !f.tt.HasText("Open Settings") {
			t.Fatalf("%s: texts %q", page, f.tt.Texts())
		}
	}
	f.shot(t, "no-reader")
	f.click(t, "Open Settings")
	f.tt.Frame()
	if !f.tt.HasText("Choose a reader…") || !f.tt.HasText("Look for readers") {
		t.Fatalf("settings: texts %q", f.tt.Texts())
	}
	f.shot(t, "settings-no-reader")
	f.click(t, "Choose a reader…")
	f.tt.Frame()
	f.click(t, "ACS ACR38U-CCID")
	f.until(t, func() bool { return f.g.app.Loaded })
	if f.g.app.CurrentDevice() != "ACS ACR38U-CCID" {
		t.Fatalf("chose %q", f.g.app.CurrentDevice())
	}
	// Nothing polls: a second look happens only on the button.
	f.click(t, "Look for readers")
	if !f.g.app.Discovering {
		t.Fatal("the button did not look for devices")
	}
	f.until(t, func() bool { return !f.g.app.Discovering })
}

func TestSettingsAIDCustomAndManualNotify(t *testing.T) {
	f := newFixture(t, newFakeCard())
	f.tt.Frame()
	f.chooseReader(t, "ACS ACR38U-CCID")
	f.click(t, "Settings")
	f.tt.Frame()

	// Turning automatic notifications off warns, and points at what waits.
	f.g.app.UpdateSettings(func(s *settings.Config) { s.AutoNotify = false })
	f.tt.Frame()
	if !f.tt.HasText("1 waiting on the card now.") {
		t.Fatalf("no warning; texts %q", f.tt.Texts())
	}
	f.tt.Advance(time.Second)
	f.tt.Frame()
	f.shot(t, "settings-manual")

	// Choosing Custom shows the field even though the AID is a preset's.
	if err := f.tt.Click("Standard (GSMA)"); err != nil {
		t.Fatal(err)
	}
	f.tt.Frame()
	if err := f.tt.Click(locales.SettingsAidCustom()); err != nil {
		t.Fatalf("choose custom: %v; texts %q", err, f.tt.Texts())
	}
	f.tt.Frame()
	if !f.tt.HasText("Apply") {
		t.Fatalf("no custom AID field; texts %q", f.tt.Texts())
	}
	f.g.customAID = "A0000005591010FFFFFFFF8900050500"
	f.tt.Frame()
	f.click(t, "Apply")
	f.tt.Frame()
	if f.g.app.Settings.AID != "A0000005591010FFFFFFFF8900050500" || !f.g.aidCustomMode {
		t.Fatalf("AID %s custom %v", f.g.app.Settings.AID, f.g.aidCustomMode)
	}

	f.click(t, "About")
	f.tt.Frame()
	if !f.tt.HasText("from lpac 654215e") || !f.tt.HasText("Author: creamlike1024") {
		t.Fatalf("about; texts %q", f.tt.Texts())
	}
	for _, want := range []string{"LGPL-2.1-only", "MIT", "cJSON", "MyGo", "libeuicc"} {
		if !f.tt.HasText(want) {
			t.Fatalf("license %q missing; texts %q", want, f.tt.Texts())
		}
	}
	f.shot(t, "about")
}

func TestNotificationsNewestFirstAndMultiSelect(t *testing.T) {
	card := newFakeCard()
	card.add("enable", "8985200012345678901")
	card.add("disable", "8944100098765432109")
	f := newFixture(t, card)
	f.tt.Frame()
	f.chooseReader(t, "ACS ACR38U-CCID")
	f.click(t, "Notifications")
	f.tt.Frame()

	rows := f.g.sortedNotifications()
	if rows[0].SeqNumber != 3 || rows[2].SeqNumber != 1 {
		t.Fatalf("not newest first: %d..%d", rows[0].SeqNumber, rows[2].SeqNumber)
	}
	// The # header switches the order.
	f.click(t, "#")
	f.tt.Frame()
	if rows := f.g.sortedNotifications(); rows[0].SeqNumber != 1 {
		t.Fatalf("header click did not sort ascending: %+v", f.g.notifSort)
	}

	// Cmd-click adds a row to the choice.
	if err := f.tt.Click("Enabled"); err != nil {
		t.Fatal(err)
	}
	if err := f.tt.ClickWith(ui.Cmd, "Disabled"); err != nil {
		t.Fatal(err)
	}
	f.tt.Frame()
	if got := f.g.selectedNotifications(); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("selected %v, want [2 3]", got)
	}
	f.shot(t, "notifications-multi")
	f.click(t, "Send 2 selected")
	f.settle(t)
	if len(card.notifications) != 1 || card.notifications[0].SeqNumber != 1 {
		t.Fatalf("left %+v", card.notifications)
	}
}

func TestEUMAndSiteTiles(t *testing.T) {
	// The EUM by the EID prefix, with its own country.
	if m := eumTile("89049032000001000000123456789068"); m.name != "🇩🇪 G+D" || m.detail != "89049032" || m.full != "Giesecke+Devrient GmbH" {
		t.Fatalf("eum %+v", m)
	}
	if m := eumTile("89086035000000000000000000000001"); m.name != "Unknown" {
		t.Fatalf("unknown eum %+v", m)
	}
	// The production site by the SAS accreditation number, with the site's country.
	if m := siteTile("HG-BG-UP-0926"); m.name != "🇨🇳 BHDC" || m.detail != "HG-BG-UP-0926" || m.full != "Beijing Huahong IC Design Co., Ltd." {
		t.Fatalf("site %+v", m)
	}
	if m := siteTile(""); m.name != "Unknown" {
		t.Fatalf("unknown site %+v", m)
	}
}
