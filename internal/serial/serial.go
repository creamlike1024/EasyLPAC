// Package serial abstracts serial port enumeration and byte transport.
// The concrete implementation uses go.bug.st/serial v1.8.0
// (BSD-3-Clause), pinned in go.mod.
package serial

import (
	"errors"
	"time"

	"go.bug.st/serial"
)

// ErrTimeout reports that no data arrived within the read timeout.
// go.bug.st/serial signals timeouts as (0, nil); that is normalized
// here so callers can distinguish "no data yet" from real errors.
var ErrTimeout = errors.New("serial: read timeout")

// Port is an opened serial port. Read honors the configured read
// timeout; a timeout yields a non-nil error (never a silent 0,nil).
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// OpenOptions configure one port.
type OpenOptions struct {
	Baud        int
	ReadTimeout time.Duration
}

// Defaults for modem APDU channels.
const (
	DefaultBaud        = 115200
	DefaultReadTimeout = 500 * time.Millisecond
)

// Opener creates ports; injectable for tests.
type Opener interface {
	Open(name string, opts OpenOptions) (Port, error)
	Enumerate() ([]string, error)
}

// System is the real serial implementation.
type System struct{}

func (System) Open(name string, opts OpenOptions) (Port, error) {
	if name == "" {
		return nil, errors.New("serial: empty port name")
	}
	baud := opts.Baud
	if baud == 0 {
		baud = DefaultBaud
	}
	port, err := serial.Open(name, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, err
	}
	timeout := opts.ReadTimeout
	if timeout <= 0 {
		timeout = DefaultReadTimeout
	}
	if err := port.SetReadTimeout(timeout); err != nil {
		_ = port.Close()
		return nil, err
	}
	return &timeoutPort{Port: port}, nil
}

// timeoutPort normalizes the library's (0, nil) timeout into ErrTimeout.
type timeoutPort struct{ serial.Port }

func (p *timeoutPort) Read(b []byte) (int, error) {
	n, err := p.Port.Read(b)
	if n == 0 && err == nil {
		return 0, ErrTimeout
	}
	return n, err
}

// Enumerate lists serial ports (darwin: /dev/cu.*; linux: ttyUSB/ttyACM;
// windows: COM ports), as reported by the platform enumeration of
// go.bug.st/serial.
func (System) Enumerate() ([]string, error) {
	return serial.GetPortsList()
}
