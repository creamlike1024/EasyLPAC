// Package settings holds the application's settings for one run. They
// live in memory only: EasyLPAC writes no configuration file, and every
// launch starts from Default, with no device chosen.
package settings

import "EasyLPAC/internal/transport"

// Backend identifiers.
const (
	BackendPCSC = "pcsc"
	BackendAT   = "at"
	BackendCCID = "ccid"
)

// Defaults.
const (
	DefaultAID       = "A0000005591010FFFFFFFF8900000100"
	DefaultATBaud    = 115200
	DefaultATTimeout = 60 // seconds, modem-side APDU execution
)

// AID presets shipped with the application.
const (
	AIDDefault = "A0000005591010FFFFFFFF8900000100"
	AID5ber    = "A0000005591010FFFFFFFF8900050500"
	AIDESIMme  = "A0000005591010000000008900000300"
	AIDXesim   = "A0000005591010FFFFFFFF8900000177"
)

// Backend holds the per-backend settings. Fields are the union of all
// backend options; a backend uses only its own fields.
type Backend struct {
	// Reader is the exact PC/SC reader name (pcsc), or the USB device's
	// name (ccid).
	Reader string
	// Port is the serial port path (at).
	Port string
	// Baud is the serial baud rate (at).
	Baud int
	// Timeout is the data-command timeout in seconds (at).
	Timeout int
	// DisableTerminalCapability skips the TERMINAL CAPABILITY
	// announcement (pcsc, ccid).
	DisableTerminalCapability bool
	// NoPPS keeps the card at the default rate (ccid).
	NoPPS bool
	// ATMode is how APDUs reach the card (at): ATModeCCHO, the default
	// when empty, or ATModeCSIM.
	ATMode string
	// RefreshOnSwitch requests the refresh when enabling or disabling
	// a profile (at).
	RefreshOnSwitch bool
	// InitCommands are sent before the capability probes (at).
	InitCommands []string
}

// AT modes (Backend.ATMode), as transport names them.
const (
	ATModeCCHO = transport.ATModeCCHO
	ATModeCSIM = transport.ATModeCSIM
)

// Config is the settings of the current run.
type Config struct {
	AID        string
	Backend    string
	Backends   map[string]Backend
	AutoNotify bool
	DebugAPDU  bool
	DebugHTTP  bool
	// HideIdentifiers obscures EIDs and ICCIDs in the UI and the log.
	HideIdentifiers bool
}

// Default returns the settings a launch starts with: PC/SC, no device
// chosen, the standard ISD-R AID, notifications sent automatically.
func Default() *Config {
	return &Config{
		AID:     DefaultAID,
		Backend: BackendPCSC,
		Backends: map[string]Backend{
			BackendPCSC: {},
			BackendCCID: {},
			BackendAT:   {Baud: DefaultATBaud, Timeout: DefaultATTimeout},
		},
		AutoNotify: true,
	}
}
