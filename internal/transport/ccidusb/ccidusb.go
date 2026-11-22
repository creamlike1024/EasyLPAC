// Package ccidusb is the card transport of USB CCID readers driven by
// go-ccid, without PC/SC: for systems that have none, as Android, and
// for Linux without pcscd.
//
// How readers are found and opened depends on the platform, and is a
// Source: on Linux (not Android), the readers of usbfs; on Android, the
// app hands the file descriptors of the readers it was allowed to open.
package ccidusb

import (
	"context"
	"errors"
	"fmt"

	ccid "github.com/creamlike1024/go-ccid"

	"EasyLPAC/internal/transport"
)

// Reader is a reader a Source found.
type Reader struct {
	ID    string // for Source.Open, as the device file; not durable
	Label string
}

// Source finds and opens CCID readers.
type Source interface {
	List(ctx context.Context) ([]Reader, error)
	// Open claims the reader's CCID interface and returns its transport,
	// which closes all it opened, and its class descriptor.
	Open(ctx context.Context, id string) (ccid.Transport, ccid.Descriptor, error)
}

// Backend is the USB CCID transport backend.
type Backend struct {
	src Source
}

// New returns the backend of the readers of src, or of the platform's
// source when src is nil (none outside Linux: Available is then false).
func New(src Source) *Backend {
	if src == nil {
		src = platformSource()
	}
	return &Backend{src: src}
}

func (b *Backend) ID() string { return "ccid" }

func (b *Backend) Describe() string { return "USB smart card reader (CCID)" }

// Available reports whether the platform has a way to reach readers.
func (b *Backend) Available() bool { return b.src != nil }

// Discover lists the CCID readers attached.
func (b *Backend) Discover(ctx context.Context) ([]transport.Device, error) {
	if b.src == nil {
		return nil, errors.New("ccid: no USB access on this platform")
	}
	readers, err := b.src.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("ccid: listing readers: %w", err)
	}
	devices := make([]transport.Device, 0, len(readers))
	for _, r := range readers {
		devices = append(devices, transport.Device{ID: r.ID, Label: r.Label})
	}
	return devices, nil
}

// Open connects to the card in the configured reader's first slot.
func (b *Backend) Open(ctx context.Context, cfg transport.DeviceConfig) (transport.CardSession, error) {
	if cfg.CCID == nil || cfg.CCID.Reader == "" {
		return nil, errors.New("ccid: no reader configured")
	}
	if b.src == nil {
		return nil, errors.New("ccid: no USB access on this platform")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t, desc, err := b.src.Open(ctx, cfg.CCID.Reader)
	if err != nil {
		return nil, fmt.Errorf("ccid: opening %q: %w", cfg.CCID.Reader, err)
	}
	r, err := ccid.Open(t, desc, nil)
	if err != nil {
		t.Close()
		return nil, fmt.Errorf("ccid: %w", err)
	}
	// The status words come back as the card sends them, as with PC/SC:
	// libeuicc fetches the responses of 61xx itself.
	card, err := r.Connect(ctx, 0, &ccid.ConnectOptions{NoPPS: cfg.CCID.NoPPS})
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("ccid: connecting to the card: %w", err)
	}
	s := &session{reader: r, card: card}
	if !cfg.CCID.DisableTerminalCapability {
		// Compatibility announcement; response ignored by design, see
		// transport.PCSCConfig.
		_, _ = s.card.Transmit(ctx, transport.TerminalCapabilities)
	}
	return s, nil
}

// session implements transport.CardSession.
type session struct {
	reader *ccid.Reader
	card   *ccid.Card
	closed bool
}

func (s *session) Transmit(ctx context.Context, apdu []byte) ([]byte, error) {
	if s.closed {
		return nil, errors.New("ccid: session closed")
	}
	resp, err := s.card.Transmit(ctx, apdu)
	if err != nil {
		return nil, fmt.Errorf("ccid: transmit: %w", err)
	}
	return resp, nil
}

func (s *session) tx(ctx context.Context) transport.TransmitFunc {
	return func(apdu []byte) ([]byte, error) { return s.Transmit(ctx, apdu) }
}

func (s *session) OpenLogicalChannel(ctx context.Context, aid []byte) (uint8, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return transport.OpenChannel(s.tx(ctx), aid, "ccid")
}

func (s *session) CloseLogicalChannel(ctx context.Context, channel uint8) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return transport.CloseChannel(s.tx(ctx), channel, "ccid")
}

// Close powers the card off and releases the reader. Idempotent.
func (s *session) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	err := s.card.Disconnect(context.Background())
	if cerr := s.reader.Close(); err == nil {
		err = cerr
	}
	return err
}
