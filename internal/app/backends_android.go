//go:build android

package app

import (
	"EasyLPAC/internal/settings"
	"EasyLPAC/internal/transport"
	"EasyLPAC/internal/transport/ccidusb"
)

// SupportedBackends are the backends this build offers: Android has
// neither PC/SC nor serial ports an app may open.
var SupportedBackends = []string{settings.BackendCCID}

// newCCIDBackend is the backend of USB CCID readers, through go-ccid.
func newCCIDBackend() transport.Backend { return ccidusb.New(nil) }
