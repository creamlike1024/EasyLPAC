//go:build android

package app

import "EasyLPAC/internal/settings"

// SupportedBackends are the backends this build offers: Android has
// neither PC/SC nor serial ports an app may open.
var SupportedBackends = []string{settings.BackendCCID}
