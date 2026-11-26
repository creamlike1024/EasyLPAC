//go:build !android

package app

import (
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
)

// SupportedBackends are the backends this build offers, the default
// first.
var SupportedBackends = []string{settings.BackendPCSC, settings.BackendAT}

// newCCIDBackend is none: desktops read cards through their systems'
// PC/SC, and their builds leave go-ccid out.
func newCCIDBackend() transport.Backend { return nil }
