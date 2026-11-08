package euiccsvc

// These tests run the real vendored libeuicc over a scripted mock card:
// the mock answers APDUs at the wire level (DER responses + SW), which
// exercises the whole chain (service -> euicc context -> shim -> libeuicc
// parsing -> domain mapping) without hardware.

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"EasyLPAC/internal/euicc"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/transport"
)

// Fixed identifiers come from internal/testcard.
var (
	testAID    = testcard.AID
	testICCID  = testcard.ICCID
	testICCID2 = testcard.ICCID2
)

// mockCard implements transport.CardSession on top of testcard.Card
// (scripted ES10 responses) and records session lifecycle.
type mockCard struct {
	card     testcard.Card
	closed   bool
	channels []uint8
	failEID  bool
}

func (m *mockCard) OpenLogicalChannel(_ context.Context, aid []byte) (uint8, error) {
	if len(aid) != len(testAID) {
		return 0, errors.New("unexpected aid")
	}
	m.channels = append(m.channels, 1)
	return 1, nil
}

func (m *mockCard) CloseLogicalChannel(_ context.Context, ch uint8) error {
	m.channels = nil
	return nil
}

func (m *mockCard) Close() error { m.closed = true; return nil }

func (m *mockCard) Transmit(_ context.Context, apdu []byte) ([]byte, error) {
	m.card.FailEID = m.failEID
	return m.card.Respond(apdu)
}

// mockBackend implements transport.Backend.
type mockBackend struct {
	card       *mockCard
	openErr    error
	discovered []transport.Device
}

func (b *mockBackend) ID() string       { return "pcsc" }
func (b *mockBackend) Describe() string { return "mock" }
func (b *mockBackend) Available() bool  { return true }

func (b *mockBackend) Discover(context.Context) ([]transport.Device, error) {
	return b.discovered, nil
}

func (b *mockBackend) Open(context.Context, transport.DeviceConfig) (transport.CardSession, error) {
	if b.openErr != nil {
		return nil, b.openErr
	}
	return b.card, nil
}

func newService(t *testing.T, b *mockBackend) *Service {
	t.Helper()
	svc, err := New(Config{
		Backend: b,
		Device: transport.DeviceConfig{
			BackendID: "pcsc",
			PCSC:      &transport.PCSCConfig{Reader: "Mock Reader"},
		},
		AID: testAID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestChipInfo(t *testing.T) {
	card := &mockCard{}
	svc := newService(t, &mockBackend{card: card})

	info, err := svc.ChipInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.EidValue != strings.ToLower("890102030405060708090A0B0C0D0E0F") {
		t.Fatalf("EID = %q", info.EidValue)
	}
	if info.EuiccConfiguredAddresses.DefaultDpAddress != "smdp.example.com" {
		t.Fatalf("default DP = %v", info.EuiccConfiguredAddresses.DefaultDpAddress)
	}
	if info.EuiccConfiguredAddresses.RootDsAddress != "smds.example.com" {
		t.Fatalf("root DS = %v", info.EuiccConfiguredAddresses.RootDsAddress)
	}
	i2 := info.EUICCInfo2
	if i2.ProfileVersion != "2.2.2" || i2.Svn != "2.2.0" || i2.EuiccFirmwareVer != "1.0.1" {
		t.Fatalf("info2 versions = %+v", i2)
	}
	if i2.ExtCardResource.FreeNonVolatileMemory != 0x001000 {
		t.Fatalf("free NV = %x", i2.ExtCardResource.FreeNonVolatileMemory)
	}
	if i2.Ts102241Version != "2.2.1" || i2.GlobalplatformVersion != "2.3.0" {
		t.Fatalf("versions = %+v", i2)
	}
	if i2.EuiccCategory != "basicEuicc" {
		t.Fatalf("category = %q", i2.EuiccCategory)
	}
	if i2.SasAcreditationNumber != "SAS-1234" {
		t.Fatalf("sas = %q", i2.SasAcreditationNumber)
	}
	if i2.CertificationDataObject.DiscoveryBaseURL != "https://ds.example" {
		t.Fatalf("discovery URL = %q", i2.CertificationDataObject.DiscoveryBaseURL)
	}
	if !card.closed {
		t.Fatal("session not closed after operation")
	}
}

func TestProfileList(t *testing.T) {
	card := &mockCard{}
	svc := newService(t, &mockBackend{card: card})

	profiles, err := svc.ProfileList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles", len(profiles))
	}
	p := profiles[0]
	if p.Iccid != testICCID || p.ProfileState != "enabled" || p.ProfileClass != "operational" {
		t.Fatalf("profile0 = %+v", p)
	}
	if p.ProfileNickname == nil || *p.ProfileNickname != "MyProfile" {
		t.Fatalf("nickname = %v", p.ProfileNickname)
	}
	if p.ServiceProviderName != "Carrier Inc" || p.ProfileName != "Carrier Profile" {
		t.Fatalf("names = %+v", p)
	}
	if p.IconType != "png" || string(p.Icon) != "\x89PNG" {
		t.Fatalf("icon = %q type %q", p.Icon, p.IconType)
	}
	if profiles[1].Iccid != testICCID2 || profiles[1].ProfileState != "disabled" ||
		profiles[1].ProfileNickname != nil {
		t.Fatalf("profile1 = %+v", profiles[1])
	}
}

func TestNotificationList(t *testing.T) {
	card := &mockCard{}
	svc := newService(t, &mockBackend{card: card})

	notifications, err := svc.NotificationList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 1 {
		t.Fatalf("got %d notifications", len(notifications))
	}
	n := notifications[0]
	if n.SeqNumber != 7 || n.ProfileManagementOperation != "install" ||
		n.NotificationAddress != "smdp.example.com" || n.Iccid != testICCID {
		t.Fatalf("notification = %+v", n)
	}
}

func TestChipInfoEIDFailureClosesSession(t *testing.T) {
	card := &mockCard{failEID: true}
	svc := newService(t, &mockBackend{card: card})

	_, err := svc.ChipInfo(context.Background())
	if err == nil {
		t.Fatal("expected EID failure")
	}
	if !strings.Contains(err.Error(), "chip-info") {
		t.Fatalf("err = %v", err)
	}
	if !card.closed {
		t.Fatal("session must be released after failed operation")
	}
}

func TestConnectFailureIsStructured(t *testing.T) {
	svc := newService(t, &mockBackend{openErr: errors.New("no card")})
	_, err := svc.ChipInfo(context.Background())
	opErr, ok := err.(*lpa.OpError)
	if !ok {
		t.Fatalf("err type %T: %v", err, err)
	}
	if opErr.Stage != lpa.StageConnect || opErr.BackendID != "pcsc" {
		t.Fatalf("opErr = %+v", opErr)
	}
}

func TestDeviceDiscovery(t *testing.T) {
	svc := newService(t, &mockBackend{
		card:       &mockCard{},
		discovered: []transport.Device{{ID: "Reader A", Label: "Reader A"}},
	})
	devices, err := svc.Devices(context.Background(), "pcsc")
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Env != "Reader A" {
		t.Fatalf("devices = %+v", devices)
	}
	if _, err := svc.Devices(context.Background(), "at"); err == nil {
		t.Fatal("expected unknown backend error")
	}
}

// LpacCommit names the commit the lpac submodule points at.
func TestLpacCommitMatchesSubmodule(t *testing.T) {
	out, err := exec.Command("git", "-C", "../../..", "ls-files", "--stage", "third_party/lpac").Output()
	if err != nil {
		t.Skipf("no git repository: %v", err)
	}
	// "160000 <sha> 0\tthird_party/lpac": a gitlink, the submodule's commit
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "160000" {
		t.Fatalf("third_party/lpac is not a submodule: %q", out)
	}
	if fields[1] != euicc.LpacCommit {
		t.Fatalf("submodule at %s, euicc.LpacCommit %s", fields[1], euicc.LpacCommit)
	}
	if !strings.HasPrefix(euicc.LpacCommit, euicc.LpacShortCommit) || len(euicc.LpacShortCommit) != 7 {
		t.Fatalf("short commit %q is not the first 7 characters of %s", euicc.LpacShortCommit, euicc.LpacCommit)
	}
}
