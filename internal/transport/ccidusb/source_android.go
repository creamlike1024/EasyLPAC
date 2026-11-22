//go:build android

package ccidusb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	ccid "github.com/creamlike1024/go-ccid"
	"github.com/creamlike1024/go-ccid/usbfs"

	"EasyLPAC/internal/usbhost"
)

func platformSource() Source { return androidSource{} }

var host = sync.OnceValues(usbhost.NewHost)

// androidSource reaches the readers through Android's UsbManager, and
// speaks to them through usbfs on the connection's file descriptor.
type androidSource struct{}

func (androidSource) List(ctx context.Context) ([]Reader, error) {
	h, err := host()
	if err != nil {
		return nil, err
	}
	found, err := h.Readers()
	if err != nil {
		return nil, err
	}
	readers := make([]Reader, 0, len(found))
	for _, r := range found {
		name := strings.TrimSpace(r.Manufacturer + " " + r.Product)
		if name == "" {
			name = "USB reader"
		}
		readers = append(readers, Reader{ID: r.Name, Label: fmt.Sprintf("%s (%04x:%04x)", name, r.VendorID, r.ProductID)})
	}
	return readers, nil
}

func (androidSource) Open(ctx context.Context, id string) (ccid.Transport, ccid.Descriptor, error) {
	var none ccid.Descriptor
	h, err := host()
	if err != nil {
		return nil, none, err
	}
	conn, err := h.Open(ctx, id)
	if err != nil {
		return nil, none, err
	}
	fd, err := conn.FD()
	if err != nil {
		return nil, none, errors.Join(err, conn.Close())
	}
	dev, err := usbfs.FromFD(fd, nil)
	if err != nil {
		return nil, none, errors.Join(err, conn.Close())
	}
	ifs := dev.Interfaces()
	if len(ifs) == 0 {
		return nil, none, errors.Join(fmt.Errorf("%s: %w", id, ccid.ErrNoInterface), dev.Close(), conn.Close())
	}
	ep, err := dev.Claim(ifs[0])
	if err != nil {
		return nil, none, errors.Join(err, dev.Close(), conn.Close())
	}
	return connTransport{ep, dev, conn}, ifs[0].Descriptor, nil
}

// connTransport closes the connection after what uses its descriptor.
type connTransport struct {
	*usbfs.Endpoints
	dev  *usbfs.Device
	conn usbhost.Connection
}

func (t connTransport) Close() error {
	return errors.Join(t.Endpoints.Close(), t.dev.Close(), t.conn.Close())
}
