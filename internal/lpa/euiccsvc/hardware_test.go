package euiccsvc

// Hardware verification entry point. Requires a real PC/SC reader with
// an eUICC and is skipped unless EASYLPAC_HW_READER is set:
//
//	EASYLPAC_HW_READER="<reader name>" go test ./internal/lpa/euiccsvc/ -run Hardware -v
//
// Read-only by design; no card state is modified. The EID is masked in
// the log output.

import (
	"context"
	"os"
	"testing"

	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/at"
	"EasyLPAC/internal/transport/pcsc"
)

var defaultAID = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF,
	0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}

func TestHardwareRead(t *testing.T) {
	reader := os.Getenv("EASYLPAC_HW_READER")
	if reader == "" {
		t.Skip("set EASYLPAC_HW_READER to a PC/SC reader name to run hardware verification")
	}

	backend := pcsc.New()
	if !backend.Available() {
		t.Fatal("PC/SC unavailable")
	}
	devices, err := backend.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	found := false
	for _, d := range devices {
		if d.ID == reader {
			found = true
		}
	}
	if !found {
		t.Fatalf("reader %q not in %v", reader, devices)
	}

	svc, err := New(Config{
		Backend: backend,
		Device:  transport.DeviceConfig{BackendID: "pcsc", PCSC: &transport.PCSCConfig{Reader: reader}},
		AID:     defaultAID,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	info, err := svc.ChipInfo(ctx)
	if err != nil {
		t.Fatalf("chip info: %v", err)
	}
	t.Logf("EID: %s...", mask(info.EidValue))
	t.Logf("default SM-DP+: %v root SM-DS: %v", info.EuiccConfiguredAddresses.DefaultDpAddress,
		info.EuiccConfiguredAddresses.RootDsAddress)
	t.Logf("EUICCInfo2: profileVersion=%s svn=%s firmware=%s",
		info.EUICCInfo2.ProfileVersion, info.EUICCInfo2.Svn, info.EUICCInfo2.EuiccFirmwareVer)

	profiles, err := svc.ProfileList(ctx)
	if err != nil {
		t.Fatalf("profile list: %v", err)
	}
	t.Logf("profiles: %d", len(profiles))
	for _, p := range profiles {
		n := "(no nickname)"
		if p.ProfileNickname != nil {
			n = *p.ProfileNickname
		}
		t.Logf("  ICCID %s state=%s class=%s nickname=%s provider=%s",
			mask(p.Iccid), p.ProfileState, p.ProfileClass, n, p.ServiceProviderName)
	}

	notifications, err := svc.NotificationList(ctx)
	if err != nil {
		t.Fatalf("notification list: %v", err)
	}
	t.Logf("notifications: %d", len(notifications))

	// A second read proves the session is released and the card is
	// usable again (no leaked channel/context).
	if _, err := svc.ChipInfo(ctx); err != nil {
		t.Fatalf("second chip info: %v", err)
	}
}

func mask(s string) string {
	if len(s) <= 7 {
		return s
	}
	return s[:7] + "****"
}

// TestHardwareATRead verifies the AT read paths against a real modem
// port. Requires a modem that supports CCHO/CCHC/CGLA:
//
//	EASYLPAC_HW_AT_PORT=/dev/cu.usbmodemXXXX go test ./internal/lpa/euiccsvc/ -run HardwareAT -v
//
// Read-only; no modem state (radio, PIN, slots) is modified.
func TestHardwareATRead(t *testing.T) {
	port := os.Getenv("EASYLPAC_HW_AT_PORT")
	if port == "" {
		t.Skip("set EASYLPAC_HW_AT_PORT to run AT hardware verification")
	}
	svc, err := New(Config{
		Backend: at.New(),
		Device: transport.DeviceConfig{
			BackendID: "at",
			AT:        &transport.ATConfig{Port: port, Baud: 115200, Timeout: 60},
		},
		AID: defaultAID,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	info, err := svc.ChipInfo(ctx)
	if err != nil {
		t.Fatalf("chip info over AT: %v", err)
	}
	t.Logf("EID: %s...", mask(info.EidValue))
	profiles, err := svc.ProfileList(ctx)
	if err != nil {
		t.Fatalf("profile list over AT: %v", err)
	}
	t.Logf("profiles: %d", len(profiles))
	notifications, err := svc.NotificationList(ctx)
	if err != nil {
		t.Fatalf("notification list over AT: %v", err)
	}
	t.Logf("notifications: %d", len(notifications))
}
