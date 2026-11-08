package demo

import (
	"context"
	"strings"
	"testing"
	"time"

	"EasyLPAC/internal/app"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/settings"
)

// newApp drives the simulated card through the real app layer, without
// delays, its dispatches run by drain.
func newApp(t *testing.T) (*app.App, *Card, func()) {
	t.Helper()
	card := New()
	card.Speed = 0
	q := make(chan func(), 64)
	cfg := settings.Default()
	cfg.Backends[settings.BackendPCSC] = settings.Backend{Reader: ReaderName}
	cfg.AutoNotify = true
	a := app.New(app.Options{
		Settings:   cfg,
		Dispatch:   func(fn func()) { q <- fn },
		NewService: func(_ *settings.Config, onStage func(string)) (lpa.Service, error) { return card.Service(onStage), nil },
		Discover:   Discover,
	})
	drain := func() {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for a.Busy() {
			select {
			case fn := <-q:
				fn()
				if a.Confirm != nil {
					a.AnswerConfirm(true)
				}
			case <-deadline:
				t.Fatal("task did not finish")
			}
		}
	}
	return a, card, drain
}

func TestCardIsConsistent(t *testing.T) {
	a, _, drain := newApp(t)
	_ = a.Refresh()
	drain()
	if !a.Loaded || len(a.Profiles) != 10 || len(a.Notifications) != 2 || !app.ValidEID(a.Chip.EidValue) {
		t.Fatalf("loaded=%v profiles=%d notifications=%d eid valid=%v", a.Loaded, len(a.Profiles), len(a.Notifications), app.ValidEID(a.Chip.EidValue))
	}
	// Turning on the second profile turns the first off: the enable and
	// the disable notifications are both sent, the older ones stay.
	_ = a.SetProfileEnabled("8944110000000000025", true)
	drain()
	if a.Profiles[0].ProfileState != "disabled" || a.Profiles[1].ProfileState != "enabled" {
		t.Fatalf("states %s %s", a.Profiles[0].ProfileState, a.Profiles[1].ProfileState)
	}
	if len(a.Notifications) != 2 {
		t.Fatalf("notifications %d, want the 2 that were pending", len(a.Notifications))
	}
	// Sending all: the unreachable server's stays.
	var seqs []int
	for _, n := range a.Notifications {
		seqs = append(seqs, n.SeqNumber)
	}
	_ = a.ProcessNotifications(seqs, true)
	drain()
	if len(a.Notifications) != 1 || a.Notifications[0].NotificationAddress != "unreachable.invalid" {
		t.Fatalf("left %+v", a.Notifications)
	}
}

func TestDownloadScenarios(t *testing.T) {
	for _, tc := range []struct {
		matchID, confirm string
		ok               bool
		want             string
	}{
		{"DEMO-OK", "", true, ""},
		{"DEMO-CONFIRM", "1234", true, ""},
		{"DEMO-CONFIRM", "", false, "8.2.7"},
		{"DEMO-CONFIRM", "0000", false, "8.2.7"},
		{"DEMO-REFUSED", "", false, "8.2.6"},
		{"DEMO-EXPIRED", "", false, "8.8.5"},
		{"DEMO-FULL", "", false, "Insufficient"},
	} {
		a, card, drain := newApp(t)
		before := len(card.profiles)
		var res *app.DownloadResult
		_ = a.Download(lpa.PullInfo{SMDP: "smdp.demo.example", MatchID: tc.matchID, ConfirmCode: tc.confirm}, func(r *app.DownloadResult) { res = r })
		drain()
		if tc.ok {
			if res == nil || !res.NotificationSent || len(card.profiles) != before+1 {
				t.Errorf("%s: res %+v err %v", tc.matchID, res, a.LastError)
			}
			continue
		}
		if res != nil || a.LastError == nil || !strings.Contains(app.DescribeError(a.LastError)+a.LastError.Error(), tc.want) {
			t.Errorf("%s/%s: res %+v err %v", tc.matchID, tc.confirm, res, a.LastError)
		}
		if _, _, isServer := app.ServerError(a.LastError); strings.HasPrefix(tc.want, "8.") && !isServer {
			t.Errorf("%s: not an SM-DP+ error: %v", tc.matchID, a.LastError)
		}
		if len(card.profiles) != before {
			t.Errorf("%s: a profile was installed", tc.matchID)
		}
	}
}

func TestDownloadCancel(t *testing.T) {
	card := New()
	card.Speed = 1
	svc := card.Service(nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, err := svc.ProfileDownload(ctx, lpa.DownloadRequest{PullInfo: lpa.PullInfo{SMDP: "smdp.demo.example", MatchID: "DEMO-SLOW"}})
	if err == nil || ctx.Err() == nil {
		t.Fatalf("err %v", err)
	}
	if len(card.profiles) != 10 {
		t.Fatal("cancelled download installed a profile")
	}
}
