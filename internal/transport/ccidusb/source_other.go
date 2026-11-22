//go:build !linux

package ccidusb

// platformSource is none: readers are reached on Linux and Android only,
// or through New with a Source.
func platformSource() Source { return nil }
