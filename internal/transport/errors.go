package transport

import "errors"

// Kinds of failure the backends tell apart, which the interface explains
// in words of its own: errors.Is finds them in what the backends return.
var (
	// ErrAccessDenied: the user did not allow the app to use the device.
	ErrAccessDenied = errors.New("access to the device denied")
	// ErrDeviceGone: the device was unplugged, before or while it was used.
	ErrDeviceGone = errors.New("device disconnected")
	// ErrNoCard: the reader has no card in it.
	ErrNoCard = errors.New("no card in the reader")
	// ErrCardMute: the card does not answer.
	ErrCardMute = errors.New("card does not answer")
)

// Kind gives err the kind of failure kind, which errors.Is then finds,
// keeping err's message.
func Kind(kind, err error) error {
	if err == nil || errors.Is(err, kind) {
		return err
	}
	return &kindError{kind, err}
}

type kindError struct{ kind, err error }

func (e *kindError) Error() string   { return e.err.Error() }
func (e *kindError) Unwrap() []error { return []error{e.err, e.kind} }
