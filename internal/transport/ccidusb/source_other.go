//go:build !linux || android

package ccidusb

// platformSource is none: Android's readers come from the app, through
// New with a Source of their file descriptors.
func platformSource() Source { return nil }
