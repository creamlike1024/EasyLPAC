// Package at implements the AT (CCHO/CCHC/CGLA) card transport,
// following 3GPP TS 27.007 §8.46-8.47 semantics:
//
//	AT+CCHO="<aid hex>"            open channel + select AID (modem side)
//	AT+CGLA=<session>,<len>,"<hex>" exchange one raw APDU
//	AT+CCHC=<session>              close channel
//
// The modem session identifier returned by +CCHO is opaque and is NOT
// the APDU logical channel number; the session keeps its own mapping
// from a small channel number (1-3, what libeuicc encodes in the CLA
// low nibble) to the modem identifier.
//
// at_csim is out of scope by decision (Q2): only the CCHO mode is
// implemented here.
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
func (b *Backend) Describe() string { return "AT modem (CCHO/CCHC/CGLA)" }

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

// Open connects and runs the non-destructive handshake: "AT", then
// capability probes for CCHO/CCHC/CGLA. It never resets the modem,
// changes radio state or touches other applications' channels.
func (b *Backend) Open(ctx context.Context, cfg transport.DeviceConfig) (transport.CardSession, error) {
	if cfg.AT == nil || cfg.AT.Port == "" {
		return nil, errors.New("at: no port configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
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
		byChannel:   make(map[uint8]string),
	}

	if err := s.handshake(); err != nil {
		_ = port.Close()
		return nil, err
	}
	return s, nil
}

type session struct {
	// Exactly one client owns the port: a second line buffer on the
	// same stream would corrupt responses. Per-command timeouts are
	// passed to CommandWithTimeout.
	client      *atclient.Client
	dataTimeout time.Duration

	mu        sync.Mutex
	closed    bool
	byChannel map[uint8]string // channel number -> modem session id
}

func (s *session) handshake() error {
	if _, err := s.client.Command("AT", ""); err != nil {
		return fmt.Errorf("at: no response to AT: %w", err)
	}
	for _, probe := range []string{"AT+CCHO=?", "AT+CCHC=?", "AT+CGLA=?"} {
		if _, err := s.client.Command(probe, ""); err != nil {
			return fmt.Errorf("at: modem does not support %s: %w", strings.TrimSuffix(probe, "=?"), err)
		}
	}
	return nil
}

func (s *session) Transmit(ctx context.Context, apdu []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("at: session closed")
	}

	// libeuicc sets the channel in the CLA low nibble; look the
	// mapping up by that channel number.
	channel := apdu[0] & 0x0F
	sessionID, ok := s.byChannel[channel]
	if !ok {
		return nil, fmt.Errorf("at: no open channel for CLA channel %d", channel)
	}

	hexAPDU := atclient.EncodeHex(apdu)
	cmd := fmt.Sprintf("AT+CGLA=%s,%d,\"%s\"", sessionID, len(hexAPDU), hexAPDU)
	res, err := s.client.CommandWithTimeout(cmd, "+CGLA: ", s.dataTimeout)
	if err != nil {
		return nil, err
	}
	parsed, err := atclient.ParseCGLA(res.Value)
	if err != nil {
		return nil, err
	}
	return parsed.Response, nil
}

func (s *session) OpenLogicalChannel(ctx context.Context, aid []byte) (uint8, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(aid) == 0 || len(aid) > 32 {
		return 0, fmt.Errorf("at: invalid AID length %d", len(aid))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("at: session closed")
	}

	channel, ok := s.nextChannel()
	if !ok {
		return 0, fmt.Errorf("at: no free channel (max %d)", maxChannels)
	}

	hexAID := atclient.EncodeHex(aid)
	res, err := s.client.Command(fmt.Sprintf("AT+CCHO=\"%s\"", hexAID), "+CCHO: ")
	if err != nil {
		return 0, err
	}
	sessionID := strings.TrimSpace(res.Value)
	if sessionID == "" {
		return 0, errors.New("at: empty session identifier from +CCHO")
	}
	s.byChannel[channel] = sessionID
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
	sessionID, ok := s.byChannel[channel]
	if !ok {
		return fmt.Errorf("at: channel %d is not open", channel)
	}
	if _, err := s.client.Command("AT+CCHC="+sessionID, ""); err != nil {
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
	for ch, sessionID := range s.byChannel {
		_, _ = s.client.Command("AT+CCHC="+sessionID, "")
		delete(s.byChannel, ch)
	}
	return s.client.Close()
}
