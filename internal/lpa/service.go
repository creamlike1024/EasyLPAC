package lpa

import "context"

// Service is the boundary between the UI and any LPA implementation.
// The legacy CLI adapter and the future libeuicc implementation both
// satisfy this interface. Implementations must be safe for sequential
// use; serialization across goroutines is the caller's responsibility
// (the UI task scheduler provides it).
type Service interface {
	// Backends lists the APDU backends available in this installation.
	Backends(ctx context.Context) ([]string, error)
	// Devices lists the devices of one backend that support enumeration.
	Devices(ctx context.Context, backend string) ([]Device, error)

	ChipInfo(ctx context.Context) (*EuiccInfo, error)

	ProfileList(ctx context.Context) ([]*Profile, error)
	ProfileEnable(ctx context.Context, iccid string) error
	ProfileDisable(ctx context.Context, iccid string) error
	ProfileDelete(ctx context.Context, iccid string) error
	ProfileNickname(ctx context.Context, iccid, nickname string) error
	// ProfileDownload downloads and installs a profile. It performs no
	// UI interaction of its own: the optional req.Confirm callback is
	// invoked with the profile metadata preview before the package is
	// fetched, and may veto the install. Notification processing stays
	// the caller's responsibility after a nil error return.
	ProfileDownload(ctx context.Context, req DownloadRequest) (*DownloadOutcome, error)

	NotificationList(ctx context.Context) ([]*Notification, error)
	NotificationProcess(ctx context.Context, seq int, remove bool) error
	NotificationRemove(ctx context.Context, seq int) error

	ChipDefaultSmdp(ctx context.Context, smdp string) error

	// Version reports the underlying protocol library version.
	Version(ctx context.Context) (string, error)
}

// EventKind identifies lifecycle points of an operation.
type EventKind int

const (
	EventStarted EventKind = iota
	EventFinished
)

// Event is emitted by service implementations around each operation.
// TaskID correlates events of one operation; events from stale tasks
// must not update UI bound to a newer device/session.
type Event struct {
	TaskID int64
	Kind   EventKind
	Op     string
}
