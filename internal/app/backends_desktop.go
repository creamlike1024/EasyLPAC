//go:build !android

package app

import "EasyLPAC/internal/settings"

// SupportedBackends are the backends this build offers, the default
// first.
var SupportedBackends = []string{settings.BackendPCSC, settings.BackendAT}
