// Package transport defines the device/backend contract between the
// LPA orchestration layer and concrete card transports (pcsc, at, ...).
//
// Invariants (docs/refactor-architecture.md §2):
//   - session methods are called serially; a session is never used from
//     two goroutines at once;
//   - Open returns a connected session, or releases everything and
//     returns an error;
//   - Close is idempotent and releases the connection;
//   - context is a cancellation intent, not a guarantee of immediate
//     interruption of in-flight system calls. After a cancellation the
//     transport state is unknown; the caller must Close and reopen.
package transport

import "context"

// Backend is one card transport technology.
type Backend interface {
	// ID is the stable backend identifier ("pcsc", "at").
	ID() string
	// Describe is the human-readable name shown in the UI.
	Describe() string
	// Available reports whether this backend can be used on this
	// machine (library/platform present). A false result is not an
	// error and must not prevent explicit configuration.
	Available() bool
	// Discover lists currently attached devices. Discovery never
	// transmits to unrelated devices. A discovery failure does not
	// imply the backend is unusable (manual configuration may work).
	Discover(ctx context.Context) ([]Device, error)
	// Open connects to the device described by cfg.
	Open(ctx context.Context, cfg DeviceConfig) (CardSession, error)
}

// Device is one discovered device.
type Device struct {
	// ID is the backend-internal identifier for reconnect purposes
	// (PC/SC reader name). It is not a durable identity.
	ID string
	// Label is the display name.
	Label string
}

// DeviceConfig is the explicit per-backend configuration.
type DeviceConfig struct {
	BackendID string
	PCSC      *PCSCConfig
	AT        *ATConfig
}

// PCSCConfig configures the PC/SC backend.
type PCSCConfig struct {
	// Reader is the exact PC/SC reader name.
	Reader string
	// DisableTerminalCapability omits the TERMINAL CAPABILITY
	// announcement sent after connecting. The announcement is on by
	// default because some cards require it (upstream lpac commit
	// 1a9cb3a, "send terminal profile is required on some card");
	// its response is ignored by design.
	DisableTerminalCapability bool
}

// ATConfig configures the AT backend (P4).
type ATConfig struct {
	Port    string
	Mode    string // "ccho" | "csim"
	Baud    int
	Timeout int
}

// CardSession is a connected card. All methods are safe for sequential
// use only.
type CardSession interface {
	// OpenLogicalChannel opens a logical channel to aid and returns
	// the channel number, ready for libeuicc use after SELECT.
	OpenLogicalChannel(ctx context.Context, aid []byte) (uint8, error)
	// CloseLogicalChannel closes a channel opened by this session.
	CloseLogicalChannel(ctx context.Context, channel uint8) error
	// Transmit exchanges a complete raw APDU; the response includes
	// SW1/SW2.
	Transmit(ctx context.Context, apdu []byte) ([]byte, error)
	// Close releases the session. Idempotent.
	Close() error
}

// SW returns the status word of a response, or 0 if absent.
func SW(resp []byte) uint16 {
	if len(resp) < 2 {
		return 0
	}
	return uint16(resp[len(resp)-2])<<8 | uint16(resp[len(resp)-1])
}
