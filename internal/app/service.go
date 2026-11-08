// Package app is the UI-independent application layer. It assembles the
// in-process LPA service from the settings of the run; the workflows,
// task scheduling and state that UIs consume are added here as well
// (docs/gui-migration-plan.md).
package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"EasyLPAC/internal/euicc"
	"EasyLPAC/internal/lpa"
	"EasyLPAC/internal/lpa/euiccsvc"
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/at"
	"EasyLPAC/internal/transport/pcsc"
)

var (
	ErrBackendUnsupported  = errors.New("backend is not supported by this build")
	ErrDeviceNotConfigured = errors.New("no device configured for the selected backend")
)

// SupportedBackends are the backends this build can drive.
var SupportedBackends = []string{settings.BackendPCSC, settings.BackendAT}

// IsSupportedBackend reports whether a backend id is implemented.
func IsSupportedBackend(id string) bool {
	return slices.Contains(SupportedBackends, id)
}

// LibeuiccVersion is the version of the bundled libeuicc: the short lpac
// commit it is built from (libeuicc has no version of its own).
func LibeuiccVersion() string { return euicc.LpacShortCommit }

// LpacCommit is the full lpac commit libeuicc is built from.
func LpacCommit() string { return euicc.LpacCommit }

// NewService builds the service for the backend and device selected in
// cfg. cfg is read once; later changes do not affect the service.
func NewService(cfg *settings.Config, onStage func(string)) (lpa.Service, error) {
	return newService(cfg, onStage, nil)
}

func newService(cfg *settings.Config, onStage func(string), logger *slog.Logger) (lpa.Service, error) {
	if !IsSupportedBackend(cfg.Backend) {
		return nil, fmt.Errorf("%w: %s", ErrBackendUnsupported, cfg.Backend)
	}
	aid, err := ParseAID(cfg.AID)
	if err != nil {
		return nil, err
	}

	svcCfg := euiccsvc.Config{
		AID:       aid,
		DebugAPDU: cfg.DebugAPDU,
		DebugHTTP: cfg.DebugHTTP,
		OnStage:   onStage,
		Logger:    logger,
	}
	b := cfg.Backends[cfg.Backend]
	switch cfg.Backend {
	case settings.BackendPCSC:
		if b.Reader == "" {
			return nil, ErrDeviceNotConfigured
		}
		svcCfg.Backend = pcsc.New()
		svcCfg.Device = transport.DeviceConfig{
			BackendID: settings.BackendPCSC,
			PCSC: &transport.PCSCConfig{
				Reader:                    b.Reader,
				DisableTerminalCapability: b.DisableTerminalCapability,
			},
		}
	case settings.BackendAT:
		if b.Port == "" {
			return nil, ErrDeviceNotConfigured
		}
		svcCfg.Backend = at.New()
		svcCfg.Device = transport.DeviceConfig{
			BackendID: settings.BackendAT,
			AT: &transport.ATConfig{
				Port:    b.Port,
				Baud:    b.Baud,
				Timeout: b.Timeout,
			},
		}
	}
	return euiccsvc.New(svcCfg)
}

// DiscoverDevices lists the devices of one backend. Discovery needs no
// card session, so it talks to the backend directly.
func DiscoverDevices(ctx context.Context, backend string) ([]transport.Device, error) {
	var b transport.Backend
	switch backend {
	case settings.BackendPCSC:
		b = pcsc.New()
	case settings.BackendAT:
		b = at.New()
	default:
		return nil, fmt.Errorf("%w: %s", ErrBackendUnsupported, backend)
	}
	return b.Discover(ctx)
}

// ParseAID converts a 32-character hex ISD-R AID to bytes.
func ParseAID(s string) ([]byte, error) {
	if len(s) != 32 {
		return nil, errors.New("AID must be 32 hex characters")
	}
	aid, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.New("AID contains non-hex characters")
	}
	return aid, nil
}
