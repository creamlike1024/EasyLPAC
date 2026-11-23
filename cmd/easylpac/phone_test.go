//go:build android || phone

package main

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
)

// phoneFixture is a gui over a fake card behind a USB reader, a tester
// for each tab's window at a phone's size.
type phoneFixture struct {
	*fixture
	tabs  map[string]*ui.Tester
	got   settings.Backend // the reader's settings the service was made with
	reads atomic.Int32
}

// countingCard counts the reads of the card.
type countingCard struct {
	*fakeCard
	reads *atomic.Int32
}

func (c countingCard) ChipInfo(ctx context.Context) (*lpa.EuiccInfo, error) {
	c.reads.Add(1)
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
			return countingCard{card, &f.reads}, nil
		},
		Discover: func(context.Context, string) ([]transport.Device, error) {
			return []transport.Device{{ID: phoneReader, Label: "ACS CCID USB Reader (072f:90cc)"}}, nil
		},
	})
	for _, tab := range phoneTabs {
		f.tabs[tab.page] = ui.NewTester(f.g.pageView(tab.page), 360, 720)
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
