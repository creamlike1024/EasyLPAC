//go:build android

package ccidusb

import (
	"errors"

	"github.com/creamlike1024/go-ccid/usbfs"

	"EasyLPAC/internal/transport"
)

// usbKind gives the errors of usbfs, which Android's readers are
// reached through, their kind.
func usbKind(err error) error {
	if errors.Is(err, usbfs.ErrDisconnected) {
		return transport.Kind(transport.ErrDeviceGone, err)
	}
	return err
}
