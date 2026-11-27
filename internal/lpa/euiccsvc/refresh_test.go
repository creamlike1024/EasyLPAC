package euiccsvc

// Switching profiles with refresh over a modem (#69).

import (
	"context"
	"errors"
	"testing"
	"time"

	"EasyLPAC/internal/euicc"
	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/testmodem"
	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/at"
)

func newRefreshService(t *testing.T, o *testmodem.Opener, refresh bool) *Service {
	t.Helper()
	svc, err := New(Config{
		Backend: at.NewWithOpener(o),
		Device: transport.DeviceConfig{
			BackendID: "at",
			AT:        &transport.ATConfig{Port: "/dev/fake0", Mode: transport.ATModeCSIM, Baud: 115200, Timeout: 5},
		},
		AID:             testcard.AID,
		RefreshOnSwitch: refresh,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// shortSettle makes the wait after a refresh fit a test.
func shortSettle(t *testing.T) {
	d, w, r := settleDelay, settleWindow, settleRetry
	settleDelay, settleWindow, settleRetry = 10*time.Millisecond, 500*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { settleDelay, settleWindow, settleRetry = d, w, r })
}

// switchRequests are the refresh flags of the enable and disable
// requests the card got, in order.
func switchRequests(c *testcard.Card) []bool {
	var out []bool
	for _, r := range c.Requests {
		if testcard.HasTag(r, 0xBF31) || testcard.HasTag(r, 0xBF32) {
			out = append(out, testcard.RequestsRefresh(r))
		}
	}
	return out
}

func TestSwitchWithoutRefreshByDefault(t *testing.T) {
	m := testmodem.New()
	svc := newRefreshService(t, &testmodem.Opener{Modem: m}, false)
	if err := svc.ProfileEnable(context.Background(), testcard.ICCID2); err != nil {
		t.Fatal(err)
	}
	if got := switchRequests(m.Card()); len(got) != 1 || got[0] {
		t.Fatalf("refresh flags = %v", got)
	}
	if !svc.settleUntil.IsZero() {
		t.Fatal("no refresh, nothing to wait for")
	}
}

func TestSwitchWithRefresh(t *testing.T) {
	shortSettle(t)
	m := testmodem.New()
	o := &testmodem.Opener{Modem: m}
	svc := newRefreshService(t, o, true)
	ctx := context.Background()
	if err := svc.ProfileEnable(ctx, testcard.ICCID2); err != nil {
		t.Fatal(err)
	}
	if got := switchRequests(m.Card()); len(got) != 1 || !got[0] {
		t.Fatalf("refresh flags = %v", got)
	}

	// The modem resets the card: the port fails for a while, and the
	// read after the switch waits for it.
	o.FailOpens = 3
	if _, err := svc.ProfileList(ctx); err != nil {
		t.Fatalf("read after the refresh: %v", err)
	}
	if !svc.settleUntil.IsZero() {
		t.Fatal("the card is back: no more waiting")
	}
	// Later failures are reported at once.
	o.FailOpens = 1
	if _, err := svc.ProfileList(ctx); err == nil {
		t.Fatal("expected the open failure")
	}
}

func TestSwitchRefreshGivesUp(t *testing.T) {
	shortSettle(t)
	m := testmodem.New()
	o := &testmodem.Opener{Modem: m}
	svc := newRefreshService(t, o, true)
	if err := svc.ProfileEnable(context.Background(), testcard.ICCID2); err != nil {
		t.Fatal(err)
	}
	o.FailOpens = 1 << 30
	start := time.Now()
	if _, err := svc.ProfileList(context.Background()); err == nil {
		t.Fatal("expected the open failure")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("waited past the window")
	}
}

// A card that refuses the refresh is asked again without it, once.
func TestSwitchRefreshRefused(t *testing.T) {
	for _, code := range []uint8{euicc.ResultCatBusy, euicc.ResultCommandError} {
		m := testmodem.New()
		m.Card().RefuseRefresh = code
		svc := newRefreshService(t, &testmodem.Opener{Modem: m}, true)
		if err := svc.ProfileDisable(context.Background(), testcard.ICCID); err != nil {
			t.Fatalf("result %d: %v", code, err)
		}
		if got := switchRequests(m.Card()); len(got) != 2 || !got[0] || got[1] {
			t.Fatalf("result %d: refresh flags = %v", code, got)
		}
		if !svc.settleUntil.IsZero() {
			t.Fatalf("result %d: switched without refresh, nothing to wait for", code)
		}
	}
}

// Other result codes have nothing to do with the refresh.
func TestSwitchOtherResultNotRetried(t *testing.T) {
	m := testmodem.New()
	m.Card().WriteResult = 2 // profileNotInDisabledState
	m.Card().RefuseRefresh = 2
	svc := newRefreshService(t, &testmodem.Opener{Modem: m}, true)
	err := svc.ProfileEnable(context.Background(), testcard.ICCID)
	var res *euicc.ResultError
	if !errors.As(err, &res) || res.Code != 2 {
		t.Fatalf("err = %v", err)
	}
	if got := switchRequests(m.Card()); len(got) != 1 {
		t.Fatalf("refresh flags = %v", got)
	}
}

// A transport failure leaves the card's state unknown: no retry.
func TestSwitchTransportErrorNotRetried(t *testing.T) {
	m := testmodem.New()
	m.Card().RefuseRefresh = euicc.ResultCommandError
	svc := newRefreshService(t, &testmodem.Opener{Modem: m}, true)
	var attempts int
	m.FailAPDU = func(apdu []byte) bool {
		if len(apdu) > 5 && testcard.HasTag(apdu[5:], 0xBF31) {
			attempts++
			return true
		}
		return false
	}
	if err := svc.ProfileEnable(context.Background(), testcard.ICCID2); err == nil {
		t.Fatal("expected failure")
	}
	if attempts != 1 {
		t.Fatalf("enable sent %d times", attempts)
	}
}
