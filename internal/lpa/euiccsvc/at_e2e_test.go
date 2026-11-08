package euiccsvc

// End-to-end AT test: the service runs the real vendored libeuicc over
// the AT transport backed by a scripted modem (internal/testmodem),
// proving the same read paths work through CCHO/CCHC/CGLA without
// hardware.

import (
	"context"
	"strings"
	"testing"

	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/testmodem"
	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/at"
)

func newATService(t *testing.T, m *testmodem.Modem) *Service {
	t.Helper()
	svc, err := New(Config{
		Backend: at.NewWithOpener(&testmodem.Opener{Modem: m}),
		Device: transport.DeviceConfig{
			BackendID: "at",
			AT:        &transport.ATConfig{Port: "/dev/fake0", Baud: 115200, Timeout: 5},
		},
		AID: testcard.AID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestATReadPaths(t *testing.T) {
	m := testmodem.New()
	svc := newATService(t, m)

	info, err := svc.ChipInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.EidValue != strings.ToLower("890102030405060708090A0B0C0D0E0F") {
		t.Fatalf("EID = %q", info.EidValue)
	}
	if info.EUICCInfo2.ProfileVersion != "2.2.2" {
		t.Fatalf("info2 = %+v", info.EUICCInfo2)
	}

	profiles, err := svc.ProfileList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || profiles[0].Iccid != testcard.ICCID || profiles[0].ProfileState != "enabled" {
		t.Fatalf("profiles = %+v", profiles)
	}

	notifications, err := svc.NotificationList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 1 || notifications[0].SeqNumber != 7 ||
		notifications[0].ProfileManagementOperation != "install" {
		t.Fatalf("notifications = %+v", notifications)
	}

	// Each operation opens and closes a session; the modem must end
	// with no tracked channels and the port closed.
	var ccho, cchc int
	for _, w := range m.Writes() {
		switch {
		case strings.HasPrefix(w, "AT+CCHO="):
			ccho++
		case strings.HasPrefix(w, "AT+CCHC="):
			cchc++
		}
	}
	if ccho == 0 || ccho != cchc {
		t.Fatalf("unbalanced channel open/close: CCHO=%d CCHC=%d", ccho, cchc)
	}
	if !m.Closed() {
		t.Fatal("modem port must be closed after each operation")
	}
}

func TestATTransmitFailureIsStructured(t *testing.T) {
	m := testmodem.New()
	m.FailCGLA = true
	svc := newATService(t, m)

	_, err := svc.ChipInfo(context.Background())
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "chip-info") {
		t.Fatalf("err = %v", err)
	}
	if !m.Closed() {
		t.Fatal("port must be released after failure")
	}
}
