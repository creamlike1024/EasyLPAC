//go:build android || ios

// Package pcsc implements the PC/SC card transport; phones have no
// PC/SC, and this Backend is never available there.
package pcsc

import (
	"context"
	"errors"

	"EasyLPAC/internal/transport"
)

var errNoPCSC = errors.New("pcsc: no PC/SC on phones; use a USB CCID reader")

// Backend is the PC/SC transport backend, unavailable on phones.
type Backend struct{}

func New() *Backend { return &Backend{} }

func (b *Backend) ID() string { return "pcsc" }

func (b *Backend) Describe() string { return "PC/SC smart card reader" }

func (b *Backend) Available() bool { return false }

func (b *Backend) Discover(context.Context) ([]transport.Device, error) { return nil, errNoPCSC }

func (b *Backend) Open(context.Context, transport.DeviceConfig) (transport.CardSession, error) {
	return nil, errNoPCSC
}
