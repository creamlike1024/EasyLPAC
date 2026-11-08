// Package pcsc implements the PC/SC card transport on top of
// github.com/ebfe/scard (BSD-3-Clause).
//
// Channel logic follows ISO/IEC 7816-4 and ETSI TS 102 221: MANAGE
// CHANNEL (open P1=00 / close P1=80) plus SELECT by AID on the opened
// channel. libeuicc addresses channels with the low nibble of CLA, so
// only standard channels 1-3 are accepted here; an extended channel
// (4-19) is closed again and reported as unsupported instead of being
// silently truncated.
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
	apduTerminalCapabilities = "\x80\xAA\x00\x00\x0A\xA9\x08\x81\x00\x82\x01\x01\x83\x01\x07"
	insManageChannel         = 0x70
	insSelect                = 0xA4
)

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
	if len(aid) == 0 || len(aid) > 32 {
		return 0, fmt.Errorf("pcsc: invalid AID length %d", len(aid))
	}

	resp, err := s.transmitRaw(string([]byte{0x00, insManageChannel, 0x00, 0x00, 0x01}))
	if err != nil {
		return 0, err
	}
	if len(resp) != 3 || transport.SW(resp) != 0x9000 {
		return 0, fmt.Errorf("pcsc: MANAGE CHANNEL open failed: %x", resp)
	}
	channel := resp[0]

	// libeuicc encodes the channel in the CLA low nibble; extended
	// channels (4-19) cannot be represented that way.
	if channel < 1 || channel > 3 {
		_, _ = s.closeChannelRaw(channel)
		return 0, fmt.Errorf("pcsc: card opened unsupported logical channel %d (only 1-3 supported)", channel)
	}

	selectApdu := make([]byte, 0, 5+len(aid)+1)
	selectApdu = append(selectApdu, channel, insSelect, 0x04, 0x00, byte(len(aid)))
	selectApdu = append(selectApdu, aid...)
	selectApdu = append(selectApdu, 0x00) // Le: request full FCI

	resp, err = s.transmitRaw(string(selectApdu))
	if err != nil {
		_, _ = s.closeChannelRaw(channel)
		return 0, err
	}
	// T=0 cards answer 61xx (response bytes available) instead of
	// returning the FCI inline. The FCI is not needed, so 61xx counts
	// as success without a GET RESPONSE.
	if sw := transport.SW(resp); sw != 0x9000 && sw>>8 != 0x61 {
		_, _ = s.closeChannelRaw(channel)
		return 0, fmt.Errorf("pcsc: SELECT AID failed: SW=%04X", sw)
	}
	return channel, nil
}

// closeChannelRaw closes a logical channel; the response SW is checked
// by the caller.
func (s *session) closeChannelRaw(channel uint8) ([]byte, error) {
	return s.transmitRaw(string([]byte{0x00, insManageChannel, 0x80, channel, 0x00}))
}

func (s *session) CloseLogicalChannel(ctx context.Context, channel uint8) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	resp, err := s.closeChannelRaw(channel)
	if err != nil {
		return err
	}
	if sw := transport.SW(resp); sw != 0x9000 {
		return fmt.Errorf("pcsc: MANAGE CHANNEL close failed: SW=%04X", sw)
	}
	return nil
}

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
