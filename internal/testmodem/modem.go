// Package testmodem provides a scripted AT modem implementing the
// serial.Opener/Port interfaces, built on internal/testcard. It is
// shared by transport/at tests and end-to-end service tests; it is
// referenced only from _test.go files.
package testmodem

import (
	"errors"
	"fmt"
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

// Modem is a scripted AT modem (CCHO/CCHC/CGLA) over a byte stream.
type Modem struct {
	mu       sync.Mutex
	out      []byte
	sessions map[string]bool
	nextID   int
	card     *testcard.Card

	// Fault injection.
	BadProbe   string // probe command that answers ERROR
	FailCGLA   bool   // CGLA answers ERROR
	SilentCGLA bool   // CGLA never answers (timeout)
	FailCCHO   bool   // CCHO answers ERROR
	URC        []string

	writes []string
	closed bool
}

func New() *Modem {
	return &Modem{sessions: map[string]bool{}, card: &testcard.Card{}}
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
	default:
		m.enqueue("ERROR\r\n")
	}
	return len(p), nil
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
}

func (o *Opener) Enumerate() ([]string, error) { return o.Ports, nil }

func (o *Opener) Open(string, serial.OpenOptions) (serial.Port, error) {
	if o.Modem == nil {
		return nil, errors.New("testmodem: no modem")
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
