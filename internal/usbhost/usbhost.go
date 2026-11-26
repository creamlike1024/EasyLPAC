// Package usbhost reaches Android's USB host API, for the CCID readers
// that an app may not find or open through usbfs by itself. It is a native
// module in Kotlin, which only lists the readers, asks the user for
// permission and holds the connection open: CCID is spoken in Go, on the
// connection's file descriptor.
package usbhost

import (
	"context"

	"github.com/egoist/mygo/native"
)

var (
	// ErrDenied: the user did not allow the app to use the reader.
	ErrDenied = native.NewError("denied")
	// ErrGone: the reader is not attached, or went while it was opened.
	ErrGone = native.NewError("gone")
)

// Reader is a CCID reader attached to the device.
type Reader struct {
	// Name is the device's name, as /dev/bus/usb/001/002. It changes when
	// the reader is plugged in again.
	Name         string
	VendorID     int
	ProductID    int
	Manufacturer string
	Product      string
}

// Host is the USB host of the device.
//
//mygo:native
type Host interface {
	// Readers lists the CCID readers attached (class 0x0B, protocol 0),
	// which needs no permission.
	Readers() ([]Reader, error)
	// Open opens a reader, asking the user for permission first when the
	// app has none, and waits for the answer until ctx is done.
	Open(ctx context.Context, name string) (Connection, error)
}

// Connection is an open reader.
type Connection interface {
	// FD returns the connection's usbfs file descriptor, which is the
	// connection's: it is valid until Close and not to be closed itself.
	FD() (int, error)
	// Close closes the connection.
	Close() error
}
