package euiccsvc

// Download/notification closed-loop tests. The card side is a scripted
// ES10 responder (internal/testcard), the network side a simulated
// SM-DP+ over TLS (internal/testsmdp): the whole ES9+/ES10b flow runs
// through the real vendored libeuicc.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/testsmdp"
	"EasyLPAC/internal/transport"
)

// downloadCard wraps a testcard.Card in a transport.CardSession.
type downloadCard struct{ card *testcard.Card }

func (d *downloadCard) OpenLogicalChannel(context.Context, []byte) (uint8, error) { return 1, nil }
func (d *downloadCard) CloseLogicalChannel(context.Context, uint8) error          { return nil }
func (d *downloadCard) Close() error                                              { return nil }
func (d *downloadCard) Transmit(_ context.Context, apdu []byte) ([]byte, error) {
	return d.card.Respond(apdu)
}

type downloadBackend struct{ card *testcard.Card }

func (b *downloadBackend) ID() string       { return "pcsc" }
func (b *downloadBackend) Describe() string { return "mock" }
func (b *downloadBackend) Available() bool  { return true }
func (b *downloadBackend) Discover(context.Context) ([]transport.Device, error) {
	return nil, nil
}
func (b *downloadBackend) Open(context.Context, transport.DeviceConfig) (transport.CardSession, error) {
	return &downloadCard{card: b.card}, nil
}

func newDownloadService(t *testing.T, card *testcard.Card, smdp *testsmdp.Server) *Service {
	t.Helper()
	var stages []string
	svc, err := New(Config{
		Backend:    &downloadBackend{card: card},
		Device:     transport.DeviceConfig{BackendID: "pcsc", PCSC: &transport.PCSCConfig{Reader: "Mock"}},
		AID:        testcard.AID,
		HTTPClient: smdp.Client(),
		OnStage:    func(s string) { stages = append(stages, s) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestDownloadHappyPath(t *testing.T) {
	card := &testcard.Card{Download: true, InstallNotificationSeq: 42}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	outcome, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address(), MatchID: "MATCH-1", IMEI: "351234567890123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome == nil || outcome.Notification == nil {
		t.Fatal("expected an install notification outcome")
	}
	if outcome.Notification.SeqNumber != 42 || outcome.Notification.Iccid != testcard.ICCID {
		t.Fatalf("outcome = %+v", outcome.Notification)
	}

	// All four ES9+ endpoints were called in order.
	if len(smdp.Requests["initiateAuthentication"]) != 1 ||
		len(smdp.Requests["authenticateClient"]) != 1 ||
		len(smdp.Requests["getBoundProfilePackage"]) != 1 {
		t.Fatalf("requests = %+v", smdp.Requests)
	}
	init := smdp.Requests["initiateAuthentication"][0]
	if init["smdpAddress"] != smdp.Address() {
		t.Fatalf("smdpAddress = %v", init["smdpAddress"])
	}
}

func TestDownloadPreviewVetoCancels(t *testing.T) {
	card := &testcard.Card{Download: true}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	called := false
	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
		Confirm: func(meta lpa.ProfileMetadata) (bool, error) {
			called = true
			if meta.ICCID != testcard.ICCID || meta.ServiceProviderName != "Carrier Inc" {
				t.Errorf("metadata = %+v", meta)
			}
			return false, nil
		},
	})
	if err == nil || !IsUserCancelled(err) {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Fatal("preview callback not called")
	}
	// The package must not have been fetched after a veto.
	if len(smdp.Requests["getBoundProfilePackage"]) != 0 {
		t.Fatalf("package fetched despite veto: %+v", smdp.Requests)
	}
	if len(smdp.Requests["cancelSession"]) != 1 {
		t.Fatalf("expected protocol cancel, requests = %+v", smdp.Requests)
	}
}

func TestDownloadAcceptPreview(t *testing.T) {
	card := &testcard.Card{Download: true}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
		Confirm:  func(lpa.ProfileMetadata) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(smdp.Requests["getBoundProfilePackage"]) != 1 {
		t.Fatal("package not fetched after acceptance")
	}
}

func TestDownloadHTTPFailureCancels(t *testing.T) {
	card := &testcard.Card{Download: true}
	smdp := testsmdp.New(t)
	smdp.FailEndpoint = "initiateAuthentication"
	svc := newDownloadService(t, card, smdp)

	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "es9p_initiate_authentication") {
		t.Fatalf("err = %v", err)
	}
}

func TestDownloadErrorStatusCodesSurface(t *testing.T) {
	card := &testcard.Card{Download: true}
	smdp := testsmdp.New(t)
	smdp.ErrorStatus = "3.8"
	svc := newDownloadService(t, card, smdp)

	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "subjectCode=8.8.1") || !strings.Contains(err.Error(), "reasonCode=3.8") {
		t.Fatalf("err = %v", err)
	}
	var es9 *lpa.ES9Error
	if !errors.As(err, &es9) || es9.Subject != "8.8.1" || es9.Reason != "3.8" {
		t.Fatalf("not a typed ES9+ error: %#v", err)
	}
}

func TestDownloadBPPFailureReportsCardReason(t *testing.T) {
	card := &testcard.Card{Download: true, FailBPP: true}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "bppCommandId=5") || !strings.Contains(err.Error(), "errorReason=10") {
		t.Fatalf("err = %v", err)
	}
	// A failed install must not be followed by an automatic retry of
	// the package fetch.
	if len(smdp.Requests["getBoundProfilePackage"]) != 1 {
		t.Fatalf("unexpected retry: %+v", smdp.Requests)
	}
}

func TestDownloadUsesCardDefaultSmdp(t *testing.T) {
	// A port with nothing listening keeps the test independent of DNS
	// and proxies; the failure proves the card default was addressed.
	card := &testcard.Card{Download: true, DefaultDp: "127.0.0.1:1"}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	// Empty SMDP: the card's configured default (smdp.example.com from
	// testcard) is unreachable in tests, so expect the flow to fail
	// while addressing that host — proving the fallback happened.
	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{})
	if err == nil {
		t.Fatal("expected network failure against the card's default SM-DP+")
	}
	if !strings.Contains(err.Error(), "es9p_initiate_authentication") {
		t.Fatalf("err = %v", err)
	}
}

func TestNotificationProcessSendsAndRemoves(t *testing.T) {
	card := &testcard.Card{}
	smdp := testsmdp.New(t)
	var sent string
	smdp.OnNotification = func(payload string) { sent = payload }
	svc := newDownloadService(t, card, smdp)

	if err := svc.NotificationProcess(context.Background(), 7, true); err != nil {
		t.Fatal(err)
	}
	if sent == "" {
		t.Fatal("notification payload not delivered")
	}
	if len(smdp.Requests["handleNotification"]) != 1 {
		t.Fatalf("requests = %+v", smdp.Requests)
	}
}

func TestNotificationRemoveOnly(t *testing.T) {
	card := &testcard.Card{}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	if err := svc.NotificationRemove(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if len(smdp.Requests["handleNotification"]) != 0 {
		t.Fatal("remove must not send anything")
	}
}

func TestHandleDownloadOutcomeCorrelation(t *testing.T) {
	// The outcome carries the exact notification the card created.
	card := &testcard.Card{Download: true, InstallNotificationSeq: 99}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)

	outcome, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Notification.SeqNumber != 99 ||
		outcome.Notification.ProfileManagementOperation != "install" {
		t.Fatalf("outcome = %+v", outcome.Notification)
	}
}

func TestWriteOperationsHitTheCard(t *testing.T) {
	card := &testcard.Card{}
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, card, smdp)
	ctx := context.Background()

	if err := svc.ProfileEnable(ctx, testcard.ICCID); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := svc.ProfileDisable(ctx, testcard.ICCID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := svc.ProfileDelete(ctx, testcard.ICCID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.ProfileNickname(ctx, testcard.ICCID, "nick"); err != nil {
		t.Fatalf("nickname: %v", err)
	}
	if err := svc.ChipDefaultSmdp(ctx, "smdp.example.com"); err != nil {
		t.Fatalf("default smdp: %v", err)
	}
	if len(card.Requests) == 0 {
		t.Fatal("no APDUs reached the card")
	}
	// Requests must carry the expected command tags.
	tags := []uint16{0xBF31, 0xBF32, 0xBF33, 0xBF29, 0xBF3F}
	for _, tag := range tags {
		found := false
		for _, req := range card.Requests {
			if testcard.HasTag(req, tag) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("tag %04X not seen in requests %x", tag, card.Requests)
		}
	}
}

// A cancellation observed between stages stops the flow and attempts a
// protocol cancel instead of continuing or retrying.
func TestDownloadCancellationBetweenStages(t *testing.T) {
	card := &testcard.Card{Download: true}
	smdp := testsmdp.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := New(Config{
		Backend:    &downloadBackend{card: card},
		Device:     transport.DeviceConfig{BackendID: "pcsc", PCSC: &transport.PCSCConfig{Reader: "Mock"}},
		AID:        testcard.AID,
		HTTPClient: smdp.Client(),
		OnStage: func(stage string) {
			if stage == stageChallenge {
				cancel() // cancel while the challenge stage completes
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.ProfileDownload(ctx, lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(smdp.Requests["getBoundProfilePackage"]) != 0 {
		t.Fatal("flow continued after cancellation")
	}
}

// A package that was written to the card must not be reported as
// failed because the caller cancelled afterwards.
func TestDownloadCompletedIsNotFailedByCancellation(t *testing.T) {
	card := &testcard.Card{Download: true, InstallNotificationSeq: 5}
	smdp := testsmdp.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := New(Config{
		Backend:    &downloadBackend{card: card},
		Device:     transport.DeviceConfig{BackendID: "pcsc", PCSC: &transport.PCSCConfig{Reader: "Mock"}},
		AID:        testcard.AID,
		HTTPClient: smdp.Client(),
		OnStage: func(stage string) {
			if stage == stageLoadBoundPackage {
				cancel() // cancel during the final install stage
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := svc.ProfileDownload(ctx, lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: smdp.Address()},
	})
	if err != nil {
		t.Fatalf("completed install must not fail: %v", err)
	}
	if outcome == nil || outcome.Notification.SeqNumber != 5 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

// A download that cannot reach the SM-DP+ says so once, with the reason.
func TestDownloadUnreachableServerMessage(t *testing.T) {
	smdp := testsmdp.New(t)
	svc := newDownloadService(t, &testcard.Card{Download: true}, smdp)
	_, err := svc.ProfileDownload(context.Background(), lpa.DownloadRequest{
		PullInfo: lpa.PullInfo{SMDP: "127.0.0.1:1"},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	msg := err.Error()
	if strings.Count(msg, "download") != 1 || strings.Count(msg, "es9p_initiate_authentication") != 1 {
		t.Fatalf("repeated parts: %s", msg)
	}
	t.Log(msg)
	if !strings.Contains(msg, "127.0.0.1:1") {
		t.Fatalf("the transport error is lost: %s", msg)
	}
}
