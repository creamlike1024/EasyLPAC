package pcsc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ebfe/scard"

	"EasyLPAC/internal/transport"
)

// fakeCard scripts Transmit responses keyed by INS byte, recording the
// exact APDUs seen.
type fakeCard struct {
	seen      []string
	responder func(apdu []byte) ([]byte, error)
	closed    bool
	disconn   scard.Disposition
}

func (f *fakeCard) ActiveProtocol() scard.Protocol { return scard.ProtocolT0 }

func (f *fakeCard) Transmit(cmd []byte) ([]byte, error) {
	f.seen = append(f.seen, string(cmd))
	if f.responder != nil {
		return f.responder(cmd)
	}
	return nil, errors.New("no responder")
}

func (f *fakeCard) Disconnect(d scard.Disposition) error {
	f.closed = true
	f.disconn = d
	return nil
}

type fakeClient struct {
	readers  []string
	card     *fakeCard
	connErr  error
	released bool
}

func (f *fakeClient) ListReaders() ([]string, error) { return f.readers, nil }

func (f *fakeClient) Connect(reader string, _ scard.ShareMode, _ scard.Protocol) (card, error) {
	if f.connErr != nil {
		return nil, f.connErr
	}
	return f.card, nil
}

func (f *fakeClient) Release() error { f.released = true; return nil }

func newTestBackend(cl *fakeClient) *Backend {
	return &Backend{establish: func() (client, error) { return cl, nil }}
}

// managedCard answers MANAGE CHANNEL and SELECT per script.
func managedCard(channel byte) *fakeCard {
	return &fakeCard{responder: func(cmd []byte) ([]byte, error) {
		switch cmd[1] {
		case insManageChannel:
			if cmd[2] == 0x00 { // open
				return []byte{channel, 0x90, 0x00}, nil
			}
			return []byte{0x90, 0x00}, nil // close
		case insSelect:
			return []byte{0x6F, 0x04, 0x90, 0x00}, nil
		}
		return []byte{0x6D, 0x00}, nil
	}}
}

var testAID = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF,
	0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}

func TestDiscover(t *testing.T) {
	b := newTestBackend(&fakeClient{readers: []string{"Reader A", "", "Reader B"}})
	devices, err := b.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].ID != "Reader A" || devices[1].ID != "Reader B" {
		t.Fatalf("devices = %+v", devices)
	}
}

func TestOpenSendsTerminalCapabilityThenSelect(t *testing.T) {
	fc := managedCard(1)
	cl := &fakeClient{card: fc}
	b := newTestBackend(cl)

	s, err := b.Open(context.Background(), transport.DeviceConfig{
		BackendID: "pcsc",
		PCSC:      &transport.PCSCConfig{Reader: "Reader A"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// First APDU must be the terminal capability announcement.
	if len(fc.seen) != 1 || fc.seen[0] != apduTerminalCapabilities {
		t.Fatalf("expected terminal capability first, got %q", fc.seen)
	}

	ch, err := s.OpenLogicalChannel(context.Background(), testAID)
	if err != nil {
		t.Fatal(err)
	}
	if ch != 1 {
		t.Fatalf("channel = %d", ch)
	}

	wantSelect := string(append(append([]byte{1, insSelect, 0x04, 0x00, byte(len(testAID))}, testAID...), 0x00))
	if fc.seen[2] != wantSelect {
		t.Fatalf("SELECT APDU mismatch:\n got %x\nwant %x", fc.seen[2], wantSelect)
	}

	if err := s.CloseLogicalChannel(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	wantClose := string([]byte{0x00, insManageChannel, 0x80, ch, 0x00})
	if fc.seen[3] != wantClose {
		t.Fatalf("close APDU mismatch: %x", fc.seen[3])
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if !fc.closed || !cl.released {
		t.Fatal("Close must disconnect card and release context")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestTerminalCapabilityCanBeDisabled(t *testing.T) {
	fc := managedCard(1)
	b := newTestBackend(&fakeClient{card: fc})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(fc.seen) != 0 {
		t.Fatalf("expected no APDUs, got %q", fc.seen)
	}
}

func TestOpenLogicalChannelExtendedRejected(t *testing.T) {
	fc := managedCard(7) // extended channel
	b := newTestBackend(&fakeClient{card: fc})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.OpenLogicalChannel(context.Background(), testAID)
	if err == nil || !strings.Contains(err.Error(), "unsupported logical channel 7") {
		t.Fatalf("err = %v", err)
	}
	// The channel must have been closed again.
	if len(fc.seen) != 2 || fc.seen[1] != string([]byte{0x00, insManageChannel, 0x80, 7, 0x00}) {
		t.Fatalf("expected cleanup close, got %q", fc.seen)
	}
}

// T=0 cards answer SELECT with 61xx; observed on a real card behind an
// ACS ACR38U-CCID (SW=6125).
func TestSelectResponseBytesAvailableAccepted(t *testing.T) {
	fc := &fakeCard{responder: func(cmd []byte) ([]byte, error) {
		switch cmd[1] {
		case insManageChannel:
			return []byte{1, 0x90, 0x00}, nil
		case insSelect:
			return []byte{0x61, 0x25}, nil
		}
		return nil, errors.New("unexpected")
	}}
	b := newTestBackend(&fakeClient{card: fc})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ch, err := s.OpenLogicalChannel(context.Background(), testAID)
	if err != nil || ch != 1 {
		t.Fatalf("channel = %d, err = %v", ch, err)
	}
	if len(fc.seen) != 2 {
		t.Fatalf("expected open+select only, got %q", fc.seen)
	}
}

func TestSelectFailureClosesChannel(t *testing.T) {
	fc := &fakeCard{responder: func(cmd []byte) ([]byte, error) {
		switch cmd[1] {
		case insManageChannel:
			return []byte{1, 0x90, 0x00}, nil
		case insSelect:
			return []byte{0x6A, 0x82}, nil
		}
		return nil, errors.New("unexpected")
	}}
	b := newTestBackend(&fakeClient{card: fc})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.OpenLogicalChannel(context.Background(), testAID); err == nil {
		t.Fatal("expected SELECT failure")
	}
	if len(fc.seen) != 3 {
		t.Fatalf("expected open+select+cleanup close, got %q", fc.seen)
	}
}

func TestTransmitAfterCloseFails(t *testing.T) {
	b := newTestBackend(&fakeClient{card: managedCard(1)})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), []byte{0x00, 0x70, 0x00, 0x00, 0x01}); err == nil {
		t.Fatal("expected transmit-after-close error")
	}
}

func TestOpenRequiresReader(t *testing.T) {
	b := newTestBackend(&fakeClient{card: managedCard(1)})
	if _, err := b.Open(context.Background(), transport.DeviceConfig{}); err == nil {
		t.Fatal("expected error without reader")
	}
}

func TestContextCancellation(t *testing.T) {
	b := newTestBackend(&fakeClient{card: managedCard(1)})
	s, err := b.Open(context.Background(), transport.DeviceConfig{
		PCSC: &transport.PCSCConfig{Reader: "R", DisableTerminalCapability: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.OpenLogicalChannel(ctx, testAID); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
