//go:build !android

package ccidusb

// platformSource is none: readers are reached on Android only, or
// through New with a Source.
func platformSource() Source { return nil }
