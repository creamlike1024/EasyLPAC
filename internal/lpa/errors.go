package lpa

import (
	"errors"
	"fmt"
)

// ErrUserCancelled reports an operation the user declined (for example
// a download preview veto). Callers should not treat it as a failure.
var ErrUserCancelled = errors.New("cancelled by user")

// IsUserCancelled reports whether err was a user cancellation.
func IsUserCancelled(err error) bool { return errors.Is(err, ErrUserCancelled) }

// Stage identifies the phase of an operation for error reporting.
type Stage string

const (
	StageDiscovery       Stage = "discovery"
	StageConnect         Stage = "connect"
	StageChipInfo        Stage = "chip-info"
	StageProfileList     Stage = "profile-list"
	StageProfileEnable   Stage = "profile-enable"
	StageProfileDisable  Stage = "profile-disable"
	StageProfileDelete   Stage = "profile-delete"
	StageProfileNickname Stage = "profile-nickname"
	StageDownload        Stage = "download"
	StageNotifyList      Stage = "notification-list"
	StageNotifyProcess   Stage = "notification-process"
	StageNotifyRemove    Stage = "notification-remove"
	StageSetDefaultSmdp  Stage = "set-default-smdp"
)

// OpError carries structured failure context. See
// docs/refactor-architecture.md §6.
type OpError struct {
	Stage      Stage
	BackendID  string
	Op         string
	Cause      error
	Retryable  bool
	StateKnown bool // false = card state unknown, reconcile before retry
}

func (e *OpError) Error() string {
	if e.Op == "" || e.Op == string(e.Stage) {
		return fmt.Sprintf("%s: %v", e.Stage, e.Cause)
	}
	return fmt.Sprintf("%s: %s: %v", e.Stage, e.Op, e.Cause)
}

func (e *OpError) Unwrap() error { return e.Cause }

// ES9Error is an SM-DP+ (ES9+) function execution status reporting a
// failure: the SGP.22 subject and reason codes and the server's message.
type ES9Error struct {
	// Stage is the download stage that failed.
	Stage   string
	Subject string
	Reason  string
	Message string
	// Cause is the error the protocol library reported, if any.
	Cause error
}

func (e *ES9Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v (subjectCode=%s reasonCode=%s: %s)", e.Stage, e.Cause, e.Subject, e.Reason, e.Message)
	}
	return fmt.Sprintf("%s: ES9+ error (subjectCode=%s reasonCode=%s: %s)", e.Stage, e.Subject, e.Reason, e.Message)
}

func (e *ES9Error) Unwrap() error { return e.Cause }
