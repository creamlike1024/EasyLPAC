package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
)

// fakeCard is an lpa.Service over in-memory card state.
type fakeCard struct {
	mu            sync.Mutex
	profiles      []*lpa.Profile
	notifications []*lpa.Notification
	nextSeq       int
	sent          []int
	failSend      bool
	meta          lpa.ProfileMetadata
	confirmed     *bool
}

func (f *fakeCard) Backends(context.Context) ([]string, error)            { return nil, nil }
func (f *fakeCard) Devices(context.Context, string) ([]lpa.Device, error) { return nil, nil }
func (f *fakeCard) Version(context.Context) (string, error)               { return "test", nil }
func (f *fakeCard) ChipDefaultSmdp(context.Context, string) error         { return nil }
func (f *fakeCard) ProfileNickname(_ context.Context, _, _ string) error  { return nil }
func (f *fakeCard) NotificationRemove(_ context.Context, seq int) error   { f.drop(seq); return nil }
func (f *fakeCard) ChipInfo(context.Context) (*lpa.EuiccInfo, error) {
	return &lpa.EuiccInfo{EidValue: "89086030202200000000000000000001"}, nil
}
func (f *fakeCard) ProfileList(context.Context) ([]*lpa.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*lpa.Profile(nil), f.profiles...), nil
}
func (f *fakeCard) NotificationList(context.Context) ([]*lpa.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*lpa.Notification(nil), f.notifications...), nil
}

func (f *fakeCard) addNotification(op, iccid string) *lpa.Notification {
	f.nextSeq++
	n := &lpa.Notification{SeqNumber: f.nextSeq, ProfileManagementOperation: op, Iccid: iccid, NotificationAddress: "smdp.example"}
	f.notifications = append(f.notifications, n)
	return n
}

func (f *fakeCard) drop(seq int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, n := range f.notifications {
		if n.SeqNumber == seq {
			f.notifications = append(f.notifications[:i], f.notifications[i+1:]...)
			return
		}
	}
}

func (f *fakeCard) setState(iccid, state, op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		if p.Iccid == iccid {
			p.ProfileState = state
		}
	}
	f.addNotification(op, iccid)
	return nil
}

func (f *fakeCard) ProfileEnable(_ context.Context, iccid string) error {
	return f.setState(iccid, "enabled", "enable")
}
func (f *fakeCard) ProfileDisable(_ context.Context, iccid string) error {
	return f.setState(iccid, "disabled", "disable")
}
func (f *fakeCard) ProfileDelete(_ context.Context, iccid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profiles = slicesDeleteICCID(f.profiles, iccid)
	f.addNotification("delete", iccid)
	return nil
}

func slicesDeleteICCID(ps []*lpa.Profile, iccid string) []*lpa.Profile {
	out := ps[:0]
	for _, p := range ps {
		if p.Iccid != iccid {
			out = append(out, p)
		}
	}
	return out
}

func (f *fakeCard) NotificationProcess(_ context.Context, seq int, remove bool) error {
	if f.failSend {
		return context.DeadlineExceeded
	}
	f.mu.Lock()
	f.sent = append(f.sent, seq)
	f.mu.Unlock()
	if remove {
		f.drop(seq)
	}
	return nil
}

func (f *fakeCard) ProfileDownload(_ context.Context, req lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	ok, err := req.Confirm(f.meta)
	f.confirmed = &ok
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &lpa.OpError{Stage: lpa.StageDownload, Op: "preview", Cause: lpa.ErrUserCancelled}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profiles = append(f.profiles, &lpa.Profile{Iccid: f.meta.ICCID, ProfileState: "disabled"})
	n := f.addNotification("install", f.meta.ICCID)
	return &lpa.DownloadOutcome{Notification: n}, nil
}

// harness drives an App whose dispatches are queued and run by wait,
// standing in for the UI thread.
type harness struct {
	a    *App
	card *fakeCard
	q    chan func()
}

func newHarness(t *testing.T, card *fakeCard) *harness {
	t.Helper()
	h := &harness{card: card, q: make(chan func(), 64)}
	cfg := settings.Default()
	cfg.Backends[settings.BackendPCSC] = settings.Backend{Reader: "Reader A"}
	cfg.AutoNotify = true
	h.a = New(Options{Settings: cfg, Dispatch: func(fn func()) { h.q <- fn }, Version: "test"})
	h.a.newService = func(*settings.Config, func(string)) (lpa.Service, error) { return card, nil }
	h.a.discover = func(context.Context, string) ([]transport.Device, error) {
		return []transport.Device{{ID: "Reader A", Label: "Reader A"}}, nil
	}
	return h
}

// wait runs dispatched functions until the task is done.
func (h *harness) wait(t *testing.T) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for h.a.Task != nil {
		select {
		case fn := <-h.q:
			fn()
		case <-deadline:
			t.Fatal("task did not finish")
		}
	}
}

func TestRefreshLoadsCard(t *testing.T) {
	card := &fakeCard{profiles: []*lpa.Profile{{Iccid: "8901000000000000001", ProfileState: "enabled"}}}
	card.addNotification("install", "8901000000000000001")
	h := newHarness(t, card)
	if err := h.a.Refresh(); err != nil {
		t.Fatal(err)
	}
	if err := h.a.Refresh(); err != ErrBusy {
		t.Fatalf("second task: err = %v, want ErrBusy", err)
	}
	h.wait(t)
	if !h.a.Loaded || len(h.a.Profiles) != 1 || len(h.a.Notifications) != 1 || h.a.Chip == nil {
		t.Fatalf("state: loaded=%v profiles=%d notifications=%d", h.a.Loaded, len(h.a.Profiles), len(h.a.Notifications))
	}
}

func TestEnableSendsOnlyNewNotifications(t *testing.T) {
	card := &fakeCard{profiles: []*lpa.Profile{
		{Iccid: "8901000000000000001", ProfileState: "disabled"},
		{Iccid: "8901000000000000002", ProfileState: "disabled"},
	}}
	old := card.addNotification("install", "8901000000000000001") // pending from before
	h := newHarness(t, card)

	if err := h.a.SetProfileEnabled("8901000000000000001", true); err != nil {
		t.Fatal(err)
	}
	h.wait(t)
	if len(card.sent) != 1 || card.sent[0] == old.SeqNumber {
		t.Fatalf("sent %v, want only the new enable notification", card.sent)
	}
	if len(h.a.Notifications) != 1 || h.a.Notifications[0].SeqNumber != old.SeqNumber {
		t.Fatalf("remaining notifications %+v", h.a.Notifications)
	}
	if h.a.Profiles[0].ProfileState != "enabled" {
		t.Fatal("profile list not reloaded")
	}
	if n := h.a.TakeNotices(); len(n) != 1 || n[0].IsError() || n[0].Kind != NoticeDone || n[0].Task != TaskEnable || n[0].Sent != 1 {
		t.Fatalf("notices %+v", n)
	}
}

func TestAutoNotifyOffKeepsNotifications(t *testing.T) {
	card := &fakeCard{profiles: []*lpa.Profile{{Iccid: "8901000000000000001", ProfileState: "disabled"}}}
	h := newHarness(t, card)
	h.a.Settings.AutoNotify = false
	if err := h.a.DeleteProfile("8901000000000000001"); err != nil {
		t.Fatal(err)
	}
	h.wait(t)
	if len(card.sent) != 0 || len(h.a.Notifications) != 1 || len(h.a.Profiles) != 0 {
		t.Fatalf("sent=%v notifications=%d profiles=%d", card.sent, len(h.a.Notifications), len(h.a.Profiles))
	}
}

func TestNotificationFailureIsReportedNotFatal(t *testing.T) {
	card := &fakeCard{profiles: []*lpa.Profile{{Iccid: "8901000000000000001", ProfileState: "enabled"}}, failSend: true}
	h := newHarness(t, card)
	if err := h.a.SetProfileEnabled("8901000000000000001", false); err != nil {
		t.Fatal(err)
	}
	h.wait(t)
	if h.a.LastError != nil {
		t.Fatalf("write succeeded, but LastError = %v", h.a.LastError)
	}
	n := h.a.TakeNotices()
	if len(n) != 1 || n[0].Kind != NoticeDone || n[0].Unsent != 1 {
		t.Fatalf("notices %+v", n)
	}
}

func TestDownloadConfirmAndNotify(t *testing.T) {
	card := &fakeCard{meta: lpa.ProfileMetadata{ICCID: "8901000000000000009", ProfileName: "Test"}}
	h := newHarness(t, card)
	var res *DownloadResult
	if err := h.a.Download(lpa.PullInfo{SMDP: "smdp.example", MatchID: "SECRET-MATCH"}, func(r *DownloadResult) { res = r }); err != nil {
		t.Fatal(err)
	}
	// Answer the preview as the UI would, once it shows.
	deadline := time.After(5 * time.Second)
	for h.a.Confirm == nil {
		select {
		case fn := <-h.q:
			fn()
		case <-deadline:
			t.Fatal("no preview")
		}
	}
	if h.a.Confirm.Metadata.ProfileName != "Test" {
		t.Fatalf("preview %+v", h.a.Confirm.Metadata)
	}
	h.a.AnswerConfirm(true)
	h.wait(t)
	if res == nil || !res.NotificationSent || res.ICCID != "8901000000000000009" {
		t.Fatalf("result %+v err %v", res, h.a.LastError)
	}
	if len(h.a.Profiles) != 1 || len(h.a.Notifications) != 0 {
		t.Fatalf("profiles=%d notifications=%d", len(h.a.Profiles), len(h.a.Notifications))
	}

	// The matching ID never reaches the log; the EID is shown in full,
	// or obscured when identifiers are hidden.
	const eid = "89086030202200000000000000000001"
	var full, hidden bytes.Buffer
	if err := h.a.Log.Export(&full, false); err != nil {
		t.Fatal(err)
	}
	_ = h.a.Log.Export(&hidden, true)
	if strings.Contains(full.String(), "SECRET-MATCH") || strings.Contains(hidden.String(), "SECRET-MATCH") {
		t.Fatalf("log leaks the matching ID:\n%s", full.String())
	}
	if !strings.Contains(full.String(), eid) {
		t.Fatalf("EID missing from the full log:\n%s", full.String())
	}
	if strings.Contains(hidden.String(), eid) || !strings.Contains(hidden.String(), "8908603****") {
		t.Fatalf("EID not obscured:\n%s", hidden.String())
	}
}

func TestDownloadDeclined(t *testing.T) {
	card := &fakeCard{meta: lpa.ProfileMetadata{ICCID: "8901000000000000009"}}
	h := newHarness(t, card)
	called := false
	var res *DownloadResult
	_ = h.a.Download(lpa.PullInfo{SMDP: "smdp.example"}, func(r *DownloadResult) { called, res = true, r })
	deadline := time.After(5 * time.Second)
	for h.a.Confirm == nil {
		select {
		case fn := <-h.q:
			fn()
		case <-deadline:
			t.Fatal("no preview")
		}
	}
	h.a.AnswerConfirm(false)
	h.wait(t)
	if !called || res != nil || h.a.LastError != nil || len(card.profiles) != 0 {
		t.Fatalf("called=%v res=%+v err=%v", called, res, h.a.LastError)
	}
}

func TestLogBufferLimitAndMask(t *testing.T) {
	b := NewLogBuffer(3)
	log := slog.New(b.Handler(slog.LevelInfo))
	for i := 0; i < 5; i++ {
		log.Info("line", "n", i, "iccid", "8901000000000000001")
	}
	entries, _ := b.Snapshot()
	if len(entries) != 3 || !strings.Contains(entries[0].AttrText(false), "n=2") {
		t.Fatalf("entries %+v", entries)
	}
	if !strings.Contains(entries[0].AttrText(false), "8901000000000000001") {
		t.Fatalf("iccid not kept: %s", entries[0].AttrText(false))
	}
	if strings.Contains(entries[0].AttrText(true), "8901000000000000001") || !strings.Contains(entries[0].AttrText(true), "8901000****") {
		t.Fatalf("iccid not masked: %s", entries[0].AttrText(true))
	}
	var buf bytes.Buffer
	_ = b.Export(&buf, false)
	if !strings.Contains(buf.String(), "2 older entries were dropped") {
		t.Fatalf("export:\n%s", buf.String())
	}
}

func TestDownloadFailureVisibleToCallback(t *testing.T) {
	card := &fakeCard{}
	h := newHarness(t, card)
	h.a.newService = func(*settings.Config, func(string)) (lpa.Service, error) { return failingDownload{card}, nil }
	var seen error
	called := false
	_ = h.a.Download(lpa.PullInfo{SMDP: "smdp.example"}, func(r *DownloadResult) { called, seen = true, h.a.LastError })
	h.wait(t)
	if !called || seen == nil {
		t.Fatalf("callback called=%v saw LastError=%v", called, seen)
	}
}

type failingDownload struct{ *fakeCard }

func (failingDownload) ProfileDownload(context.Context, lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	return nil, &lpa.ES9Error{Stage: "es9p_initiate_authentication", Subject: "8.2.6", Reason: "3.8"}
}

// Finding devices never chooses one, nor reads a card.
func TestDiscoveryChoosesNothing(t *testing.T) {
	h := newHarness(t, &fakeCard{})
	h.a.Settings.Backends[settings.BackendPCSC] = settings.Backend{}
	h.a.RefreshDevices()
	if !h.a.Discovering {
		t.Fatal("Discovering not set")
	}
	(<-h.q)()
	if h.a.Discovering || len(h.a.Devices) != 1 || h.a.CurrentDevice() != "" || h.a.Busy() {
		t.Fatalf("devices %v current %q busy %v", h.a.Devices, h.a.CurrentDevice(), h.a.Busy())
	}
}
