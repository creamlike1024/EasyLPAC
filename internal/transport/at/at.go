// Package at implements the AT card transport in two modes
// (transport.ATModeCCHO, transport.ATModeCSIM); see ccho.go and csim.go.
//
// Both modes share the port, the AT client, the handshake and the
// mapping from the APDU logical channel number (1-3, what libeuicc
// encodes in the CLA low nibble) to what the mode needs to address that
// channel.
package at

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"EasyLPAC/internal/atclient"
	"EasyLPAC/internal/serial"
	"EasyLPAC/internal/transport"
)

// Control command timeouts; data commands use the configured timeout.
const (
	controlTimeout = 5 * time.Second
	maxChannels    = 3 // libeuicc encodes channel in the CLA low nibble (1-3)
)

// Backend is the AT transport backend.
type Backend struct {
	opener serial.Opener
}

func New() *Backend                          { return &Backend{opener: serial.System{}} }
func NewWithOpener(o serial.Opener) *Backend { return &Backend{opener: o} }

func (b *Backend) ID() string       { return "at" }
func (b *Backend) Describe() string { return "AT modem (CCHO/CGLA or CSIM)" }

// Available is always true: the backend exists on all platforms; a
// usable port is a per-device property.
func (b *Backend) Available() bool { return true }

// Discover lists serial ports. Enumeration never transmits to any port.
func (b *Backend) Discover(ctx context.Context) ([]transport.Device, error) {
	ports, err := b.opener.Enumerate()
	if err != nil {
		return nil, fmt.Errorf("at: enumerate ports: %w", err)
	}
	devices := make([]transport.Device, 0, len(ports))
	for _, p := range ports {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if isIgnoredPort(p) {
			continue // debug/console ports must not be disturbed
		}
		devices = append(devices, transport.Device{ID: p, Label: p})
	}
	return devices, nil
}

// isIgnoredPort filters obvious non-modem ports (console/debug).
func isIgnoredPort(name string) bool {
	return strings.Contains(name, "Bluetooth") ||
		strings.Contains(name, "debug-console")
}

// ValidateInitCommand reports whether cmd may be sent as an init
// command: one complete command line starting with AT.
func ValidateInitCommand(cmd string) error {
	if strings.ContainsAny(cmd, "\r\n") {
		return fmt.Errorf("at: init command %q spans lines", cmd)
	}
	if len(cmd) < 2 || !strings.EqualFold(cmd[:2], "AT") {
		return fmt.Errorf("at: init command %q does not start with AT", cmd)
	}
	return nil
}

// Open connects and runs the handshake: "AT", the configured init
// commands, then the mode's capability probes. Apart from the init
// commands the user chose, it never resets the modem, changes radio
// state or touches other applications' channels.
func (b *Backend) Open(ctx context.Context, cfg transport.DeviceConfig) (transport.CardSession, error) {
	if cfg.AT == nil || cfg.AT.Port == "" {
		return nil, errors.New("at: no port configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var m mode
	switch cfg.AT.Mode {
	case "", transport.ATModeCCHO:
		m = &cchoMode{}
	case transport.ATModeCSIM:
		m = &csimMode{}
	default:
		return nil, fmt.Errorf("at: unknown mode %q", cfg.AT.Mode)
	}
	for _, cmd := range cfg.AT.InitCommands {
		if err := ValidateInitCommand(cmd); err != nil {
			return nil, err
		}
	}

	port, err := b.opener.Open(cfg.AT.Port, serial.OpenOptions{
		Baud:        cfg.AT.Baud,
		ReadTimeout: serial.DefaultReadTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("at: open %s: %w", cfg.AT.Port, err)
	}

	dataTimeout := time.Duration(cfg.AT.Timeout) * time.Second
	if dataTimeout <= 0 {
		dataTimeout = 60 * time.Second // modem-side APDU execution can be slow
	}

	s := &session{
		client:      atclient.New(port, atclient.Options{Timeout: controlTimeout}),
		dataTimeout: dataTimeout,
		mode:        m,
		byChannel:   make(map[uint8]string),
	}
	m.bind(s)

	if err := s.handshake(cfg.AT.InitCommands); err != nil {
		_ = port.Close()
		return nil, err
	}
	return s, nil
}

// mode is how a session reaches the card's logical channels.
type mode interface {
	bind(s *session)
	// probes are the commands whose "=?" form the modem must accept.
	probes() []string
	// open opens a channel to aid and selects it. free is the channel
	// to use when the mode chooses it; the card may choose another. It
	// returns the channel number and the handle that addresses it.
	open(aid []byte, free uint8) (channel uint8, handle string, err error)
	transmit(handle string, apdu []byte) ([]byte, error)
	close(channel uint8, handle string) error
}

type session struct {
	// Exactly one client owns the port: a second line buffer on the
	// same stream would corrupt responses. Per-command timeouts are
	// passed to CommandWithTimeout.
	client      *atclient.Client
	dataTimeout time.Duration
	mode        mode

	mu        sync.Mutex
	closed    bool
	byChannel map[uint8]string // channel number -> the mode's handle
}

func (s *session) handshake(initCommands []string) error {
	if _, err := s.client.Command("AT", ""); err != nil {
		return fmt.Errorf("at: no response to AT: %w", err)
	}
	for _, cmd := range initCommands {
		if _, err := s.client.Command(cmd, ""); err != nil {
			return fmt.Errorf("at: init command %s: %w", cmd, err)
		}
	}
	for _, cmd := range s.mode.probes() {
		if _, err := s.client.Command(cmd+"=?", ""); err != nil {
			return fmt.Errorf("at: modem does not support %s: %w", cmd, err)
		}
	}
	return nil
}

func (s *session) Transmit(ctx context.Context, apdu []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(apdu) < 4 {
		return nil, fmt.Errorf("at: APDU of %d bytes", len(apdu))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("at: session closed")
	}

	// libeuicc sets the channel in the CLA low nibble; look the
	// mapping up by that channel number.
	channel := apdu[0] & 0x0F
	handle, ok := s.byChannel[channel]
	if !ok {
		return nil, fmt.Errorf("at: no open channel for CLA channel %d", channel)
	}
	return s.mode.transmit(handle, apdu)
}

func (s *session) OpenLogicalChannel(ctx context.Context, aid []byte) (uint8, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(aid) == 0 || len(aid) > 16 {
		return 0, fmt.Errorf("at: invalid AID length %d", len(aid))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("at: session closed")
	}

	free, ok := s.nextChannel()
	if !ok {
		return 0, fmt.Errorf("at: no free channel (max %d)", maxChannels)
	}
	channel, handle, err := s.mode.open(aid, free)
	if err != nil {
		return 0, err
	}
	s.byChannel[channel] = handle
	return channel, nil
}

// nextChannel returns the first free channel number 1..maxChannels.
func (s *session) nextChannel() (uint8, bool) {
	for ch := uint8(1); ch <= maxChannels; ch++ {
		if _, used := s.byChannel[ch]; !used {
			return ch, true
		}
	}
	return 0, false
}

func (s *session) CloseLogicalChannel(ctx context.Context, channel uint8) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("at: session closed")
	}
	handle, ok := s.byChannel[channel]
	if !ok {
		return fmt.Errorf("at: channel %d is not open", channel)
	}
	if err := s.mode.close(channel, handle); err != nil {
		return err
	}
	delete(s.byChannel, channel)
	return nil
}

// Close closes tracked channels and the port. Idempotent.
func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for ch, handle := range s.byChannel {
		_ = s.mode.close(ch, handle)
		delete(s.byChannel, ch)
	}
	return s.client.Close()
}
