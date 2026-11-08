package at

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"EasyLPAC/internal/atclient"
	"EasyLPAC/internal/testcard"
	"EasyLPAC/internal/testmodem"
	"EasyLPAC/internal/transport"
)

// The scripted modem lives in internal/testmodem (shared with the
// end-to-end service tests).

func newModem() *testmodem.Modem { return testmodem.New() }
func newOpener(m *testmodem.Modem) *testmodem.Opener {
	return &testmodem.Opener{Modem: m}
}

func testDevice() transport.DeviceConfig {
	return transport.DeviceConfig{
		BackendID: "at",
		AT:        &transport.ATConfig{Port: "/dev/fake0", Baud: 115200, Timeout: 2},
	}
}

func openSession(t *testing.T, opener *testmodem.Opener) *session {
	t.Helper()
	s, err := NewWithOpener(opener).Open(context.Background(), testDevice())
	if err != nil {
		t.Fatal(err)
	}
	return s.(*session)
}

// lastCGLA returns the most recent AT+CGLA command line.
func lastCGLA(m *testmodem.Modem) string {
	var last string
	for _, w := range m.Writes() {
		if strings.HasPrefix(w, "AT+CGLA=") {
			last = w
		}
	}
	return last
}

func TestHandshakeAndProbeFailure(t *testing.T) {
	m := newModem()
	s := openSession(t, newOpener(m))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	m2 := newModem()
	m2.BadProbe = "AT+CGLA=?"
	if _, err := NewWithOpener(newOpener(m2)).Open(context.Background(), testDevice()); err == nil {
		t.Fatal("expected probe failure")
	} else if !strings.Contains(err.Error(), "AT+CGLA") {
		t.Fatalf("err = %v", err)
	}
	if !m2.Closed() {
		t.Fatal("port must be closed when the handshake fails")
	}
}

func TestDiscoverFiltersPorts(t *testing.T) {
	b := NewWithOpener(&testmodem.Opener{Ports: []string{"/dev/cu.usbmodem1", "/dev/cu.Bluetooth-Incoming-Port"}})
	devices, err := b.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ID != "/dev/cu.usbmodem1" {
		t.Fatalf("devices = %+v", devices)
	}
}

func TestOpenRequiresPort(t *testing.T) {
	if _, err := NewWithOpener(newOpener(newModem())).Open(
		context.Background(), transport.DeviceConfig{AT: &transport.ATConfig{}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestChannelMappingAndTransmit(t *testing.T) {
	m := newModem()
	s := openSession(t, newOpener(m))
	defer s.Close()

	ch, err := s.OpenLogicalChannel(context.Background(), testcard.AID)
	if err != nil {
		t.Fatal(err)
	}
	if ch != 1 {
		t.Fatalf("channel = %d", ch)
	}

	// libeuicc builds the EID request with the channel in CLA.
	apdu := append([]byte{0x81, 0xE2, 0x91, 0x00, 0x06}, testcard.DER(0xBF3E, testcard.DER(0x5C, []byte{0x5A}))...)
	resp, err := s.Transmit(context.Background(), apdu)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) < 2 || resp[len(resp)-2] != 0x90 || resp[len(resp)-1] != 0x00 {
		t.Fatalf("response = %x", resp)
	}
	if want := atclient.EncodeHex(apdu); !strings.Contains(lastCGLA(m), want) {
		t.Fatalf("CGLA arg %q does not carry the APDU %q", lastCGLA(m), want)
	}

	// Second channel maps to the second modem session id; the CLA
	// channel number and the modem session id must not be conflated.
	ch2, err := s.OpenLogicalChannel(context.Background(), testcard.AID)
	if err != nil {
		t.Fatal(err)
	}
	if ch2 != 2 {
		t.Fatalf("second channel = %d", ch2)
	}
	apdu2 := append([]byte{0x82, 0xE2, 0x91, 0x00, 0x06}, testcard.DER(0xBF3E, testcard.DER(0x5C, []byte{0x5A}))...)
	if _, err := s.Transmit(context.Background(), apdu2); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lastCGLA(m), "AT+CGLA=2,") {
		t.Fatalf("expected CGLA to address session id 2: %q", lastCGLA(m))
	}
	// First channel still addresses session 1.
	if _, err := s.Transmit(context.Background(), apdu); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lastCGLA(m), "AT+CGLA=1,") {
		t.Fatalf("expected CGLA to address session id 1: %q", lastCGLA(m))
	}

	if err := s.CloseLogicalChannel(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), apdu); err == nil {
		t.Fatal("transmit on a closed channel must fail")
	}
}

func TestTransmitRejectsUnknownChannel(t *testing.T) {
	s := openSession(t, newOpener(newModem()))
	defer s.Close()
	if _, err := s.Transmit(context.Background(), []byte{0x83, 0xE2, 0x91, 0x00}); err == nil {
		t.Fatal("expected unknown-channel error")
	}
}

func TestChannelExhaustion(t *testing.T) {
	s := openSession(t, newOpener(newModem()))
	defer s.Close()
	for i := 0; i < maxChannels; i++ {
		if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err == nil {
		t.Fatal("expected exhaustion error")
	}
	if err := s.CloseLogicalChannel(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatalf("reuse after close: %v", err)
	}
}

func TestURCsAreNotConsumedAsResults(t *testing.T) {
	m := newModem()
	m.URC = []string{"+CREG: 1", "+CGREG: 1"}
	s := openSession(t, newOpener(m))
	defer s.Close()
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	apdu := append([]byte{0x81, 0xE2, 0x91, 0x00, 0x06}, testcard.DER(0xBF3E, testcard.DER(0x5C, []byte{0x5A}))...)
	resp, err := s.Transmit(context.Background(), apdu)
	if err != nil {
		t.Fatal(err)
	}
	if resp[len(resp)-2] != 0x90 {
		t.Fatalf("response = %x", resp)
	}
}

func TestCGLAErrorPropagates(t *testing.T) {
	m := newModem()
	m.FailCGLA = true
	s := openSession(t, newOpener(m))
	defer s.Close()
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), []byte{0x81, 0xE2, 0x91, 0x00}); err == nil {
		t.Fatal("expected CGLA error")
	}
}

// A timed-out data command must not let a late response satisfy the
// next command: the session refuses further traffic.
func TestTimeoutDesynchronizes(t *testing.T) {
	m := newModem()
	m.SilentCGLA = true
	s := openSession(t, newOpener(m))
	defer s.Close()
	s.dataTimeout = 100 * time.Millisecond

	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), []byte{0x81, 0xE2, 0x91, 0x00}); err == nil {
		t.Fatal("expected timeout")
	}
	m.SilentCGLA = false
	m.Enqueue("+CGLA: 4,\"9000\"\r\nOK\r\n") // the late response

	if _, err := s.Transmit(context.Background(), []byte{0x81, 0xE2, 0x91, 0x00}); !errors.Is(err, atclient.ErrDesynchronized) {
		t.Fatalf("err = %v", err)
	}
}

func TestCloseClosesTrackedChannels(t *testing.T) {
	m := newModem()
	s := openSession(t, newOpener(m))
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	var sawCCHC bool
	for _, w := range m.Writes() {
		if strings.HasPrefix(w, "AT+CCHC=") {
			sawCCHC = true
		}
	}
	if !sawCCHC {
		t.Fatal("Close must release modem channels")
	}
	if !m.Closed() {
		t.Fatal("port not closed")
	}
}
