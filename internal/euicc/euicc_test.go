package euicc

import (
	"bytes"
	"errors"
	"testing"
)

// fakeAPDU implements APDUInterface with a scripted card. It validates
// the request shape for GetEID and answers with a fixed DER response.
type fakeAPDU struct {
	connected    bool
	channels     []uint8
	nextChannel  uint8
	lastAID      []byte
	transmitFunc func(apdu []byte) ([]byte, error)
	transmits    int
}

var testEID = []byte{0x89, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

const testEIDHex = "890102030405060708090a0b0c0d0e0f" // libeuicc hexutil is lowercase

func (f *fakeAPDU) Connect() error {
	f.connected = true
	f.nextChannel = 1
	return nil
}

func (f *fakeAPDU) Disconnect() { f.connected = false }

func (f *fakeAPDU) LogicChannelOpen(aid []byte) (uint8, error) {
	if !f.connected {
		return 0, errors.New("not connected")
	}
	ch := f.nextChannel
	f.nextChannel++
	f.channels = append(f.channels, ch)
	f.lastAID = append([]byte(nil), aid...)
	return ch, nil
}

func (f *fakeAPDU) LogicChannelClose(channel uint8) {
	for i, c := range f.channels {
		if c == channel {
			f.channels = append(f.channels[:i], f.channels[i+1:]...)
			return
		}
	}
}

func (f *fakeAPDU) Transmit(apdu []byte) ([]byte, error) {
	f.transmits++
	return f.transmitFunc(apdu)
}

// getEIDScript answers the ES10c GetEuiccData(5A) command sequence.
// Request APDU (from libeuicc): CLA 0x81 (channel 1) 0xE2 0x91 0x00 Lc
// BF3E 03 5C 01 5A 00.
func getEIDScript(apdu []byte) ([]byte, error) {
	if len(apdu) < 6 || apdu[1] != 0xE2 {
		return nil, errors.New("unexpected APDU")
	}
	body := []byte{0xBF, 0x3E, byte(len(testEID) + 2), 0x5A, byte(len(testEID))}
	body = append(body, testEID...)
	return append(body, 0x90, 0x00), nil
}

type fakeHTTP struct{ calls int }

func (f *fakeHTTP) Transmit(url string, headers []string, tx []byte) (uint32, []byte, error) {
	f.calls++
	return 200, []byte("{}"), nil
}

var testAID = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10,
	0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}

func TestContextLifecycleAndGetEID(t *testing.T) {
	apdu := &fakeAPDU{transmitFunc: getEIDScript}
	httpi := &fakeHTTP{}

	c, err := New(apdu, httpi, testAID)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !apdu.connected {
		t.Fatal("Connect not called during init")
	}
	if len(apdu.channels) != 1 {
		t.Fatalf("expected 1 open channel, got %v", apdu.channels)
	}
	if !bytes.Equal(apdu.lastAID, testAID) {
		t.Fatalf("AID mismatch: got %x", apdu.lastAID)
	}

	eid, err := c.GetEID()
	if err != nil {
		t.Fatalf("GetEID: %v", err)
	}
	if eid != testEIDHex {
		t.Fatalf("EID = %q, want %q", eid, testEIDHex)
	}

	c.Close()
	if len(apdu.channels) != 0 {
		t.Fatalf("channel not closed on Close: %v", apdu.channels)
	}
	if apdu.connected {
		t.Fatal("Disconnect not called on Close")
	}
	// Idempotent close must not double-free or re-callback.
	c.Close()
}

type failingConnect struct{ fakeAPDU }

func (f *failingConnect) Connect() error { return errors.New("boom") }

func TestInitFailureReleasesResources(t *testing.T) {
	httpi := &fakeHTTP{}
	c, err := New(&failingConnect{}, httpi, testAID)
	if err == nil {
		if c != nil {
			c.Close()
		}
		t.Fatal("expected init failure")
	}
}

type failingChannelOpen struct{ fakeAPDU }

func (f *failingChannelOpen) LogicChannelOpen(aid []byte) (uint8, error) {
	f.connected = true // connect succeeded
	return 0, errors.New("select failed")
}

func TestChannelOpenFailureDisconnects(t *testing.T) {
	f := &failingChannelOpen{}
	f.transmitFunc = getEIDScript
	c, err := New(f, &fakeHTTP{}, testAID)
	if err == nil {
		if c != nil {
			c.Close()
		}
		t.Fatal("expected channel open failure")
	}
	if f.connected {
		t.Fatal("disconnect not called after channel open failure")
	}
}

type failingTransmit struct{ fakeAPDU }

func (f *failingTransmit) Transmit(apdu []byte) ([]byte, error) {
	return nil, errors.New("transmit error")
}

func TestTransmitErrorPropagates(t *testing.T) {
	f := &failingTransmit{}
	c, err := New(f, &fakeHTTP{}, testAID)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()
	if _, err := c.GetEID(); err == nil {
		t.Fatal("expected GetEID to fail on transmit error")
	}
}
