//go:build !android

package ccidusb

// usbKind gives the errors of the USB transport their kind: there is none
// here but a Source's own.
func usbKind(err error) error { return err }
