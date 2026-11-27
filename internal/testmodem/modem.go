// Package testmodem provides a scripted AT modem implementing the
// serial.Opener/Port interfaces, built on internal/testcard. It is
// shared by transport/at tests and end-to-end service tests; it is
// referenced only from _test.go files.
package testmodem

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"EasyLPAC/internal/atclient"
	"EasyLPAC/internal/serial"
	"EasyLPAC/internal/testcard"
)

// PortTimeout mirrors serial.ErrTimeout for scripted ports.
type PortTimeout struct{}

func (PortTimeout) Error() string { return "serial: read timeout" }
func (PortTimeout) Timeout() bool { return true }

// Modem is a scripted AT modem (CCHO/CCHC/CGLA and CSIM) over a byte
// stream.
type Modem struct {
	mu       sync.Mutex
	out      []byte
	sessions map[string]bool
	nextID   int
	card     *testcard.Card
	// channels are the card's logical channels opened with MANAGE
	// CHANNEL over AT+CSIM.
	channels map[uint8]bool

	// Fault injection.
	BadProbe   string // probe command that answers ERROR
	FailCGLA   bool   // CGLA answers ERROR
	SilentCGLA bool   // CGLA never answers (timeout)
	FailCCHO   bool   // CCHO answers ERROR
	// RejectCCHO answers AT+CCHO with +CME ERROR: 100 although its
	// probe succeeds, as the HP lt4132 of #69 does.
	RejectCCHO bool
	FailCSIM   bool // CSIM answers ERROR
	// FailAPDU makes CSIM answer ERROR for the APDUs it matches.
	FailAPDU   func(apdu []byte) bool
	SilentCSIM bool // CSIM never answers (timeout)
	// SelectSW is what SELECT over CSIM answers (default 6121, as #69's
	// card does).
	SelectSW []byte
	// FirstChannel is the lowest channel MANAGE CHANNEL hands out
	// (default 1).
	FirstChannel uint8
	// InitErrors are commands that answer ERROR (the init commands).
	InitErrors []string
	URC        []string

	writes []string
	closed bool
}

func New() *Modem {
	return &Modem{sessions: map[string]bool{}, channels: map[uint8]bool{}, card: &testcard.Card{}}
}

// Card exposes the underlying scripted card (request recording etc.).
func (m *Modem) Card() *testcard.Card { return m.card }

// Enqueue injects raw modem output (used to script late responses).
func (m *Modem) Enqueue(resp string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueue(resp)
}

// enqueue appends raw bytes; caller must hold the lock.
func (m *Modem) enqueue(resp string) { m.out = append(m.out, []byte(resp)...) }

func (m *Modem) Read(p []byte) (int, error) {
	m.mu.Lock()
	if len(m.out) > 0 {
		n := copy(p, m.out)
		m.out = m.out[n:]
		m.mu.Unlock()
		return n, nil
	}
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return 0, errors.New("modem closed")
	}
	return 0, PortTimeout{}
}

func (m *Modem) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, errors.New("modem closed")
	}
	line := strings.TrimRight(string(p), "\r")
	m.writes = append(m.writes, line)
	m.enqueue(line + "\r\r\n") // echo

	switch {
	case line == "AT":
		m.enqueue("OK\r\n")
	case strings.HasSuffix(line, "=?"):
		if line == m.BadProbe {
			m.enqueue("ERROR\r\n")
		} else {
			m.enqueue("OK\r\n")
		}
	case slices.Contains(m.InitErrors, line):
		m.enqueue("ERROR\r\n")
	case strings.HasPrefix(line, "AT+CCHO=") && m.RejectCCHO:
		m.enqueue("+CME ERROR: 100\r\n")
	case strings.HasPrefix(line, "AT+CCHO="):
		if m.FailCCHO {
			m.enqueue("ERROR\r\n")
			break
		}
		m.nextID++
		id := fmt.Sprintf("%d", m.nextID)
		m.sessions[id] = true
		m.enqueue("+CCHO: " + id + "\r\nOK\r\n")
	case strings.HasPrefix(line, "AT+CCHC="):
		delete(m.sessions, strings.TrimPrefix(line, "AT+CCHC="))
		m.enqueue("OK\r\n")
	case strings.HasPrefix(line, "AT+CGLA="):
		if m.FailCGLA {
			m.enqueue("ERROR\r\n")
			break
		}
		if m.SilentCGLA {
			break
		}
		for _, u := range m.URC {
			m.enqueue(u + "\r\n")
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "AT+CGLA="), ",", 3)
		if len(parts) != 3 {
			m.enqueue("ERROR\r\n")
			break
		}
		apdu, err := atclient.DecodeHex(strings.Trim(parts[2], "\""))
		if err != nil {
			m.enqueue("+CME ERROR: 3\r\n")
			break
		}
		resp, err := m.card.Respond(apdu)
		if err != nil {
			m.enqueue("+CME ERROR: 4\r\n")
			break
		}
		hexResp := atclient.EncodeHex(resp)
		m.enqueue(fmt.Sprintf("+CGLA: %d,\"%s\"\r\nOK\r\n", len(hexResp), hexResp))
	case strings.HasPrefix(line, "AT+CSIM="):
		m.csim(strings.TrimPrefix(line, "AT+CSIM="))
	default:
		m.enqueue("OK\r\n") // other commands (init commands) succeed
	}
	return len(p), nil
}

// csim answers AT+CSIM=<len>,"<hex>"; caller must hold the lock.
func (m *Modem) csim(args string) {
	if m.FailCSIM {
		m.enqueue("ERROR\r\n")
		return
	}
	if m.SilentCSIM {
		return
	}
	for _, u := range m.URC {
		m.enqueue(u + "\r\n")
	}
	n, arg, ok := strings.Cut(args, ",")
	arg = strings.Trim(arg, "\"")
	if !ok || n != fmt.Sprint(len(arg)) {
		m.enqueue("+CME ERROR: 3\r\n")
		return
	}
	apdu, err := atclient.DecodeHex(arg)
	if err != nil || len(apdu) < 4 {
		m.enqueue("+CME ERROR: 3\r\n")
		return
	}
	if m.FailAPDU != nil && m.FailAPDU(apdu) {
		m.enqueue("ERROR\r\n")
		return
	}
	resp, err := m.respondCSIM(apdu)
	if err != nil {
		m.enqueue("+CME ERROR: 4\r\n")
		return
	}
	hexResp := atclient.EncodeHex(resp)
	m.enqueue(fmt.Sprintf("+CSIM: %d,\"%s\"\r\nOK\r\n", len(hexResp), hexResp))
}

// respondCSIM is the card's side of one APDU on any channel: MANAGE
// CHANNEL and SELECT here, the rest by the scripted card.
func (m *Modem) respondCSIM(apdu []byte) ([]byte, error) {
	channel := apdu[0] & 0x03
	switch {
	case apdu[1] == 0x70 && apdu[2] == 0x00: // MANAGE CHANNEL open
		for ch := max(m.FirstChannel, 1); ch < 20; ch++ {
			if !m.channels[ch] {
				m.channels[ch] = true
				return []byte{ch, 0x90, 0x00}, nil
			}
		}
		return []byte{0x6A, 0x81}, nil
	case apdu[1] == 0x70 && apdu[2] == 0x80: // MANAGE CHANNEL close
		if !m.channels[apdu[3]] {
			return []byte{0x68, 0x81}, nil
		}
		delete(m.channels, apdu[3])
		return []byte{0x90, 0x00}, nil
	case channel != 0 && !m.channels[channel]:
		return []byte{0x68, 0x81}, nil // logical channel not supported
	case apdu[1] == 0xA4: // SELECT
		if len(apdu) < 5 || !slices.Equal(apdu[5:], testcard.AID) {
			return []byte{0x6A, 0x82}, nil
		}
		if m.SelectSW != nil {
			return m.SelectSW, nil
		}
		return []byte{0x61, 0x21}, nil
	}
	return m.card.Respond(apdu)
}

// Channels returns the card's logical channels left open.
func (m *Modem) Channels() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.channels)
}

func (m *Modem) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Closed reports whether the port was closed.
func (m *Modem) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Writes returns the AT commands received so far.
func (m *Modem) Writes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.writes...)
}

// Opener implements serial.Opener for one modem.
type Opener struct {
	Modem *Modem
	Ports []string
	// FailOpens makes the next opens fail, as a port does while the
	// modem resets the card.
	FailOpens int
}

func (o *Opener) Enumerate() ([]string, error) { return o.Ports, nil }

func (o *Opener) Open(string, serial.OpenOptions) (serial.Port, error) {
	if o.Modem == nil {
		return nil, errors.New("testmodem: no modem")
	}
	if o.FailOpens > 0 {
		o.FailOpens--
		return nil, errors.New("testmodem: port busy")
	}
	o.Modem.reopen()
	return o.Modem, nil
}

// reopen models re-opening the device: pending bytes and channel state
// are dropped (the modem does not remember previous sessions), while
// the write log survives for assertions.
func (m *Modem) reopen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.out = nil
	m.sessions = map[string]bool{}
	m.closed = false
}
