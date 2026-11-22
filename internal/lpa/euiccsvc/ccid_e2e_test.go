package euiccsvc

// End-to-end CCID test: the service runs the real vendored libeuicc over
// the USB CCID backend (go-ccid) and a simulated reader, at TPDU level
// with T=0 (where libeuicc fetches 61xx with GET RESPONSE itself) and
// T=1, and at APDU level, without hardware.

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	ccid "github.com/creamlike1024/go-ccid"
	"github.com/creamlike1024/go-ccid/ccidtest"

	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/ccidusb"
)

// channelCard puts testcard's ES10 responder behind MANAGE CHANNEL and
// SELECT of the ISD-R, as a ccidtest.Card.
type channelCard struct {
	atr  []byte
	es10 *testcard.Card

	mu           sync.Mutex
	open, closed int
}

func (c *channelCard) ATR() []byte { return c.atr }

func (c *channelCard) Transmit(apdu []byte) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case apdu[1] == transport.InsManageChannel && apdu[2] == 0x00:
		c.open++
		return []byte{0x01, 0x90, 0x00}
	case apdu[1] == transport.InsManageChannel && apdu[2] == 0x80:
		c.closed++
		return []byte{0x90, 0x00}
	case apdu[1] == transport.InsSelect:
		if !bytes.Contains(apdu, testcard.AID) {
			return []byte{0x6A, 0x82}
		}
		return []byte{0x6F, 0x00, 0x90, 0x00}
	case apdu[1] == 0xAA: // TERMINAL CAPABILITY
		return []byte{0x90, 0x00}
	case apdu[1] == 0xE2: // STORE DATA
		// The ES10 command as libeuicc sent it: at TPDU level with T=0,
		// the simulated card gets case 4 with an Le of 00, which the
		// responder takes as its 5-byte header and body.
		resp, err := c.es10.Respond(slices.Clone(apdu))
		if err != nil {
			return []byte{0x6F, 0x00}
		}
		return resp
	}
	return []byte{0x6D, 0x00}
}

// freshSource opens a new simulated reader over the same card each time,
// as a session's Close closes its transport.
type freshSource struct {
	card *channelCard
	o    ccidtest.Options
}

func (s *freshSource) List(context.Context) ([]ccidusb.Reader, error) {
	return []ccidusb.Reader{{ID: "sim0", Label: "Simulated reader"}}, nil
}

func (s *freshSource) Open(context.Context, string) (ccid.Transport, ccid.Descriptor, error) {
	sim := ccidtest.NewReader(s.card, s.o)
	return sim, sim.Descriptor(), nil
}

func TestCCIDReadPaths(t *testing.T) {
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
		{"TPDU T=0", ccidtest.ATRT0T1, ccidtest.Options{TimeExtensions: 1}},
		{"TPDU T=1", atrT1, ccidtest.Options{T1WTX: 1}},
		{"extended APDU", ccidtest.ATRT0T1, ccidtest.Options{Level: ccid.LevelExtendedAPDU, Features: ccid.FeatureAutoParameters}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := &channelCard{atr: tc.atr, es10: &testcard.Card{}}
			svc, err := New(Config{
				Backend: ccidusb.New(&freshSource{card: card, o: tc.o}),
				Device:  transport.DeviceConfig{BackendID: "ccid", CCID: &transport.CCIDConfig{Reader: "sim0"}},
				AID:     testcard.AID,
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			info, err := svc.ChipInfo(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if info.EidValue != strings.ToLower("890102030405060708090A0B0C0D0E0F") || info.EUICCInfo2.ProfileVersion != "2.2.2" {
				t.Fatalf("EID %q, info2 %+v", info.EidValue, info.EUICCInfo2)
			}
			profiles, err := svc.ProfileList(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(profiles) != 2 || profiles[0].Iccid != testcard.ICCID || profiles[0].ProfileState != "enabled" {
				t.Fatalf("profiles = %+v", profiles)
			}
			notifications, err := svc.NotificationList(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(notifications) != 1 || notifications[0].SeqNumber != 7 {
				t.Fatalf("notifications = %+v", notifications)
			}
			if card.open == 0 || card.open != card.closed {
				t.Fatalf("unbalanced channels: %d opened, %d closed", card.open, card.closed)
			}
		})
	}
}
