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

func csimDevice() transport.DeviceConfig {
	d := testDevice()
	d.AT.Mode = transport.ATModeCSIM
	return d
}

func openCSIM(t *testing.T, m *testmodem.Modem) *session {
	t.Helper()
	s, err := NewWithOpener(newOpener(m)).Open(context.Background(), csimDevice())
	if err != nil {
		t.Fatal(err)
	}
	return s.(*session)
}

// eidAPDU is libeuicc's EID request on channel.
func eidAPDU(channel byte) []byte {
	return append([]byte{0x80 | channel, 0xE2, 0x91, 0x00, 0x06}, testcard.DER(0xBF3E, testcard.DER(0x5C, []byte{0x5A}))...)
}

// csimWrites are the AT+CSIM commands sent, without their prefix.
func csimWrites(m *testmodem.Modem) []string {
	var out []string
	for _, w := range m.Writes() {
		if v, ok := strings.CutPrefix(w, "AT+CSIM="); ok && v != "?" {
			out = append(out, v)
		}
	}
	return out
}

// The sequence #69's reporter typed by hand: MANAGE CHANNEL, SELECT
// (answered 6121), then the ES10 commands, all over AT+CSIM, and never
// AT+CCHO, which the modem rejects.
func TestCSIMSequence(t *testing.T) {
	m := newModem()
	m.RejectCCHO = true
	s := openCSIM(t, m)

	ch, err := s.OpenLogicalChannel(context.Background(), testcard.AID)
	if err != nil {
		t.Fatal(err)
	}
	if ch != 1 {
		t.Fatalf("channel = %d", ch)
	}
	resp, err := s.Transmit(context.Background(), eidAPDU(1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(atclient.EncodeHex(resp), "9000") {
		t.Fatalf("response = %x", resp)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	want := []string{
		`10,"0070000001"`,
		`42,"01a4040010a0000005591010ffffffff8900000100"`,
		`22,"81e2910006bf3e035c015a"`,
		`10,"0070800100"`,
	}
	if got := csimWrites(m); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("CSIM commands:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, w := range m.Writes() {
		if strings.HasPrefix(w, "AT+CCHO") || strings.HasPrefix(w, "AT+CGLA") {
			t.Fatalf("CSIM mode sent %q", w)
		}
	}
	if m.Channels() != 0 || !m.Closed() {
		t.Fatalf("channels left open: %d, port closed: %v", m.Channels(), m.Closed())
	}
}

func TestCSIMProbe(t *testing.T) {
	m := newModem()
	m.BadProbe = "AT+CSIM=?"
	if _, err := NewWithOpener(newOpener(m)).Open(context.Background(), csimDevice()); err == nil ||
		!strings.Contains(err.Error(), "AT+CSIM") {
		t.Fatalf("err = %v", err)
	}
	if !m.Closed() {
		t.Fatal("port must be closed when the handshake fails")
	}

	// CSIM mode needs none of the CCHO commands.
	m2 := newModem()
	m2.BadProbe = "AT+CCHO=?"
	openCSIM(t, m2).Close()
}

// The card chooses the channel; libeuicc must get the card's number,
// and the APDUs then carry it.
func TestCSIMCardChoosesChannel(t *testing.T) {
	m := newModem()
	m.FirstChannel = 2
	s := openCSIM(t, m)
	defer s.Close()
	ch, err := s.OpenLogicalChannel(context.Background(), testcard.AID)
	if err != nil || ch != 2 {
		t.Fatalf("channel = %d, %v", ch, err)
	}
	if _, err := s.Transmit(context.Background(), eidAPDU(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), eidAPDU(1)); err == nil {
		t.Fatal("transmit on a channel the card did not open must fail")
	}
}

// A channel libeuicc can't put in the CLA is given back.
func TestCSIMRejectsHighChannel(t *testing.T) {
	m := newModem()
	m.FirstChannel = 4
	s := openCSIM(t, m)
	defer s.Close()
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err == nil {
		t.Fatal("expected an error for channel 4")
	}
	if m.Channels() != 0 {
		t.Fatal("channel 4 must be closed again")
	}
}

func TestCSIMSelectStatus(t *testing.T) {
	for _, tc := range []struct {
		sw []byte
		ok bool
	}{
		{[]byte{0x90, 0x00}, true},
		{[]byte{0x61, 0x21}, true},
		{[]byte{0x6A, 0x82}, false},
	} {
		m := newModem()
		m.SelectSW = tc.sw
		s := openCSIM(t, m)
		_, err := s.OpenLogicalChannel(context.Background(), testcard.AID)
		if (err == nil) != tc.ok {
			t.Fatalf("SELECT %X: err = %v", tc.sw, err)
		}
		if !tc.ok && m.Channels() != 0 {
			t.Fatalf("SELECT %X: the channel must be closed after a failed SELECT", tc.sw)
		}
		s.Close()
	}
}

func TestCSIMURCsAndErrors(t *testing.T) {
	m := newModem()
	m.URC = []string{"+CREG: 1", "+QIND: SMS DONE"}
	s := openCSIM(t, m)
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transmit(context.Background(), eidAPDU(1)); err != nil {
		t.Fatal(err)
	}
	m.FailCSIM = true
	if _, err := s.Transmit(context.Background(), eidAPDU(1)); err == nil {
		t.Fatal("expected CSIM error")
	}
	s.Close()
}

func TestCSIMTimeoutDesynchronizes(t *testing.T) {
	m := newModem()
	s := openCSIM(t, m)
	defer s.Close()
	s.dataTimeout = 100 * time.Millisecond
	if _, err := s.OpenLogicalChannel(context.Background(), testcard.AID); err != nil {
		t.Fatal(err)
	}
	m.SilentCSIM = true
	if _, err := s.Transmit(context.Background(), eidAPDU(1)); err == nil {
		t.Fatal("expected timeout")
	}
	m.SilentCSIM = false
	m.Enqueue("+CSIM: 4,\"9000\"\r\nOK\r\n") // the late response
	if _, err := s.Transmit(context.Background(), eidAPDU(1)); !errors.Is(err, atclient.ErrDesynchronized) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnknownMode(t *testing.T) {
	d := testDevice()
	d.AT.Mode = "qmi"
	if _, err := NewWithOpener(newOpener(newModem())).Open(context.Background(), d); err == nil {
		t.Fatal("expected unknown mode error")
	}
}

func TestInitCommands(t *testing.T) {
	m := newModem()
	d := testDevice()
	d.AT.InitCommands = []string{"AT+CMEE=2", "at+cfun?"}
	s, err := NewWithOpener(newOpener(m)).Open(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	w := m.Writes()
	if len(w) < 4 || w[0] != "AT" || w[1] != "AT+CMEE=2" || w[2] != "at+cfun?" || w[3] != "AT+CCHO=?" {
		t.Fatalf("writes = %q: init commands go after AT, before the probes", w)
	}

	m2 := newModem()
	m2.InitErrors = []string{"AT+QCFG=\"x\""}
	d.AT.InitCommands = []string{"AT+QCFG=\"x\""}
	if _, err := NewWithOpener(newOpener(m2)).Open(context.Background(), d); err == nil ||
		!strings.Contains(err.Error(), "AT+QCFG") {
		t.Fatalf("err = %v", err)
	}
	if !m2.Closed() {
		t.Fatal("port must be closed when an init command fails")
	}

	for _, bad := range []string{"CMEE=2", "AT\rATZ", ""} {
		m3 := newModem()
		d.AT.InitCommands = []string{bad}
		if _, err := NewWithOpener(newOpener(m3)).Open(context.Background(), d); err == nil {
			t.Fatalf("init command %q must be refused", bad)
		}
		if len(m3.Writes()) != 0 {
			t.Fatalf("init command %q: nothing may be sent before it is checked", bad)
		}
	}
}
