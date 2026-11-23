package ccidusb

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	ccid "github.com/creamlike1024/go-ccid"
	"github.com/creamlike1024/go-ccid/ccidtest"

	"EasyLPAC/internal/transport"
)

var isdr = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}

// euiccCard opens logical channel 1, selects the ISD-R with an FCI, and
// answers GetEID; it records the commands.
type euiccCard struct {
	atr     []byte
	channel byte // the channel MANAGE CHANNEL opens
	open    bool
	seen    [][]byte
}

func (c *euiccCard) ATR() []byte { return c.atr }

func (c *euiccCard) Transmit(apdu []byte) []byte {
	c.seen = append(c.seen, slices.Clone(apdu))
	switch {
	case apdu[1] == transport.InsManageChannel && apdu[2] == 0x00:
		c.open = true
		return []byte{c.channel, 0x90, 0x00}
	case apdu[1] == transport.InsManageChannel && apdu[2] == 0x80:
		c.open = false
		return []byte{0x90, 0x00}
	case apdu[1] == transport.InsSelect && bytes.Contains(apdu, isdr):
		return []byte{0x6F, 0x02, 0x84, 0x00, 0x90, 0x00}
	case apdu[1] == transport.InsSelect:
		return []byte{0x6A, 0x82}
	case apdu[1] == 0xE2 && bytes.Contains(apdu, []byte{0xBF, 0x3E}):
		return []byte{0xBF, 0x3E, 0x03, 0x5A, 0x01, 0x89, 0x90, 0x00}
	}
	return []byte{0x90, 0x00}
}

// simSource is a Source of one simulated reader.
type simSource struct {
	sim    *ccidtest.Reader
	opened int
}

func (s *simSource) List(context.Context) ([]Reader, error) {
	return []Reader{{ID: "sim0", Label: "Simulated reader"}}, nil
}

func (s *simSource) Open(_ context.Context, id string) (ccid.Transport, ccid.Descriptor, error) {
	if id != "sim0" {
		return nil, ccid.Descriptor{}, errors.New("no such reader")
	}
	s.opened++
	return s.sim, s.sim.Descriptor(), nil
}

func openSession(t *testing.T, card *euiccCard, o ccidtest.Options, cfg transport.CCIDConfig) (transport.CardSession, *simSource) {
	t.Helper()
	src := &simSource{sim: ccidtest.NewReader(card, o)}
	if cfg.Reader == "" {
		cfg.Reader = "sim0"
	}
	s, err := New(src).Open(context.Background(), transport.DeviceConfig{BackendID: "ccid", CCID: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, src
}

func TestDiscover(t *testing.T) {
	b := New(&simSource{})
	if !b.Available() || b.ID() != "ccid" {
		t.Fatalf("backend %q available %v", b.ID(), b.Available())
	}
	devs, err := b.Discover(context.Background())
	if err != nil || len(devs) != 1 || devs[0].ID != "sim0" {
		t.Fatalf("Discover: %v %v", devs, err)
	}
}

func TestSessionOverReaders(t *testing.T) {
	atrT1 := []byte{0x3B, 0x80, 0x81, 0x31, 0x20, 0x45}
	var x byte
	for _, b := range atrT1[1:] {
		x ^= b
	}
	atrT1 = append(atrT1, x)
	for _, tc := range []struct {
		name string
		atr  []byte
		o    ccidtest.Options
	}{
		{"TPDU T=0", ccidtest.ATRT0T1, ccidtest.Options{}},
		{"TPDU T=1", atrT1, ccidtest.Options{}},
		{"short APDU", ccidtest.ATRT0T1, ccidtest.Options{Level: ccid.LevelShortAPDU, Features: ccid.FeatureAutoParameters}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := &euiccCard{atr: tc.atr, channel: 1}
			s, _ := openSession(t, card, tc.o, transport.CCIDConfig{})
			// At TPDU level with T=0, the simulated card gets case 3 with an
			// Le of 00.
			if !bytes.HasPrefix(card.seen[0], transport.TerminalCapabilities) {
				t.Errorf("first command % x, want TERMINAL CAPABILITY", card.seen[0])
			}
			ch, err := s.OpenLogicalChannel(context.Background(), isdr)
			if err != nil || ch != 1 {
				t.Fatalf("OpenLogicalChannel: %d %v", ch, err)
			}
			resp, err := s.Transmit(context.Background(), []byte{0x81, 0xE2, 0x91, 0x00, 0x06, 0xBF, 0x3E, 0x03, 0x5C, 0x01, 0x5A, 0x00})
			if err != nil || transport.SW(resp) != 0x9000 && transport.SW(resp)>>8 != 0x61 {
				t.Fatalf("GetEID: % x %v", resp, err)
			}
			if err := s.CloseLogicalChannel(context.Background(), ch); err != nil || card.open {
				t.Fatalf("CloseLogicalChannel: %v, open %v", err, card.open)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Errorf("second Close: %v", err)
			}
			if _, err := s.Transmit(context.Background(), []byte{0x00, 0x70, 0x00, 0x00, 0x01}); err == nil {
				t.Error("Transmit after Close")
			}
		})
	}
}

func TestTerminalCapabilityCanBeDisabled(t *testing.T) {
	card := &euiccCard{atr: ccidtest.ATRT0, channel: 1}
	openSession(t, card, ccidtest.Options{}, transport.CCIDConfig{DisableTerminalCapability: true})
	if len(card.seen) != 0 {
		t.Errorf("commands sent: % x", card.seen)
	}
}

func TestExtendedChannelRejected(t *testing.T) {
	card := &euiccCard{atr: ccidtest.ATRT0, channel: 7}
	s, _ := openSession(t, card, ccidtest.Options{}, transport.CCIDConfig{})
	_, err := s.OpenLogicalChannel(context.Background(), isdr)
	if err == nil || !strings.Contains(err.Error(), "ccid: card opened unsupported logical channel 7") {
		t.Fatalf("channel 7: %v", err)
	}
	if card.open {
		t.Error("channel 7 left open")
	}
}

func TestSelectFailureClosesChannel(t *testing.T) {
	card := &euiccCard{atr: ccidtest.ATRT0, channel: 2}
	s, _ := openSession(t, card, ccidtest.Options{}, transport.CCIDConfig{})
	if _, err := s.OpenLogicalChannel(context.Background(), []byte{0xA0, 0x00, 0x00, 0x00, 0x00}); err == nil || !strings.Contains(err.Error(), "SW=6A82") {
		t.Fatalf("SELECT of an unknown AID: %v", err)
	}
	if card.open {
		t.Error("channel left open")
	}
}

func TestOpenErrors(t *testing.T) {
	b := New(&simSource{sim: ccidtest.NewReader(nil, ccidtest.Options{})})
	ctx := context.Background()
	if _, err := b.Open(ctx, transport.DeviceConfig{}); err == nil {
		t.Error("Open without a reader configured")
	}
	if _, err := b.Open(ctx, transport.DeviceConfig{CCID: &transport.CCIDConfig{Reader: "nope"}}); err == nil {
		t.Error("Open of an unknown reader")
	}
	if _, err := b.Open(ctx, transport.DeviceConfig{CCID: &transport.CCIDConfig{Reader: "sim0"}}); !errors.Is(err, ccid.ErrNoCard) {
		t.Errorf("Open without a card: %v, want ccid.ErrNoCard", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := b.Open(cctx, transport.DeviceConfig{CCID: &transport.CCIDConfig{Reader: "sim0"}}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled Open: %v", err)
	}
	if New(nil).Available() != (platformSource() != nil) {
		t.Error("Available does not follow the platform source")
	}
}

// No card in the reader, before or after the session opens, is told by
// its kind, which the interface explains.
func TestNoCardKind(t *testing.T) {
	src := &simSource{sim: ccidtest.NewReader(nil, ccidtest.Options{})}
	_, err := New(src).Open(context.Background(), transport.DeviceConfig{BackendID: "ccid", CCID: &transport.CCIDConfig{Reader: "sim0"}})
	if !errors.Is(err, transport.ErrNoCard) {
		t.Fatalf("open with no card: %v", err)
	}

	s, src2 := openSession(t, &euiccCard{atr: ccidtest.ATRT0, channel: 1}, ccidtest.Options{}, transport.CCIDConfig{})
	src2.sim.Remove()
	if _, err := s.Transmit(context.Background(), []byte{0x00, 0xA4, 0x04, 0x00, 0x00}); !errors.Is(err, transport.ErrNoCard) {
		t.Fatalf("transmit with the card out: %v", err)
	}
}
