// Package pcsc implements the PC/SC card transport on top of
// github.com/ebfe/scard (BSD-3-Clause).
//
// Logical channels are opened and closed by transport.OpenChannel and
// transport.CloseChannel, as for every backend.
package pcsc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ebfe/scard"

	"EasyLPAC/internal/transport"
)

// client abstracts the scard API for tests.
type client interface {
	ListReaders() ([]string, error)
	Connect(reader string, mode scard.ShareMode, proto scard.Protocol) (card, error)
	Release() error
}

type card interface {
	ActiveProtocol() scard.Protocol
	Transmit(cmd []byte) ([]byte, error)
	Disconnect(d scard.Disposition) error
}

// scardContext adapts *scard.Context to client.
type scardContext struct{ ctx *scard.Context }

func (c *scardContext) ListReaders() ([]string, error) { return c.ctx.ListReaders() }

func (c *scardContext) Connect(reader string, mode scard.ShareMode, proto scard.Protocol) (card, error) {
	card, err := c.ctx.Connect(reader, mode, proto)
	if err != nil {
		return nil, err
	}
	return card, nil
}

func (c *scardContext) Release() error { return c.ctx.Release() }

// Backend is the PC/SC transport backend.
type Backend struct {
	// establish creates a scard context; overridable in tests.
	establish func() (client, error)
}

func New() *Backend {
	return &Backend{
		establish: func() (client, error) {
			ctx, err := scard.EstablishContext()
			if err != nil {
				return nil, err
			}
			return &scardContext{ctx: ctx}, nil
		},
	}
}

func (b *Backend) ID() string { return "pcsc" }

func (b *Backend) Describe() string { return "PC/SC smart card reader" }

// Available reports whether a PC/SC context can be established.
func (b *Backend) Available() bool {
	ctx, err := b.establish()
	if err != nil {
		return false
	}
	_ = ctx.Release()
	return true
}

// Discover lists connected readers that currently hold a card. Reader
// names are stable within one PC/SC service instance.
func (b *Backend) Discover(ctx context.Context) ([]transport.Device, error) {
	cl, err := b.establish()
	if err != nil {
		return nil, fmt.Errorf("pcsc: establish context: %w", err)
	}
	defer cl.Release()

	readers, err := cl.ListReaders()
	if err != nil {
		return nil, fmt.Errorf("pcsc: list readers: %w", err)
	}

	devices := make([]transport.Device, 0, len(readers))
	for _, r := range readers {
		if r == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		devices = append(devices, transport.Device{ID: r, Label: r})
	}
	return devices, nil
}

// Open connects to the configured reader. The negotiated protocol is
// whatever the card and reader agree on (T=0 or T=1); scard selects the
// matching PCI. libeuicc's APDU framing is protocol independent.
func (b *Backend) Open(ctx context.Context, cfg transport.DeviceConfig) (transport.CardSession, error) {
	if cfg.PCSC == nil || cfg.PCSC.Reader == "" {
		return nil, errors.New("pcsc: no reader configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cl, err := b.establish()
	if err != nil {
		return nil, fmt.Errorf("pcsc: establish context: %w", err)
	}

	crd, err := cl.Connect(cfg.PCSC.Reader, scard.ShareExclusive, scard.ProtocolAny)
	if err != nil {
		_ = cl.Release()
		return nil, fmt.Errorf("pcsc: connect %q: %w", cfg.PCSC.Reader, err)
	}

	s := &session{client: cl, card: crd}
	if !cfg.PCSC.DisableTerminalCapability {
		// Compatibility announcement; response ignored by design,
		// see transport.PCSCConfig.
		_, _ = s.transmitRaw(apduTerminalCapabilities)
	}
	return s, nil
}

// session implements transport.CardSession.
type session struct {
	client client
	card   card
	closed bool
}

const (
	insManageChannel = transport.InsManageChannel
	insSelect        = transport.InsSelect
)

var apduTerminalCapabilities = string(transport.TerminalCapabilities)

func (s *session) Transmit(ctx context.Context, apdu []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.transmitRaw(string(apdu))
}

func (s *session) transmitRaw(apdu string) ([]byte, error) {
	if s.closed {
		return nil, errors.New("pcsc: session closed")
	}
	resp, err := s.card.Transmit([]byte(apdu))
	if err != nil {
		return nil, fmt.Errorf("pcsc: transmit: %w", err)
	}
	return resp, nil
}

// OpenLogicalChannel performs MANAGE CHANNEL (open) followed by
// SELECT by AID on the new channel.
func (s *session) OpenLogicalChannel(ctx context.Context, aid []byte) (uint8, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return transport.OpenChannel(s.tx, aid, "pcsc")
}

func (s *session) CloseLogicalChannel(ctx context.Context, channel uint8) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return transport.CloseChannel(s.tx, channel, "pcsc")
}

func (s *session) tx(apdu []byte) ([]byte, error) { return s.transmitRaw(string(apdu)) }

func (s *session) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.card != nil {
		err = s.card.Disconnect(scard.LeaveCard)
	}
	if s.client != nil {
		if rerr := s.client.Release(); rerr != nil && err == nil {
			err = rerr
		}
	}
	return err
}
