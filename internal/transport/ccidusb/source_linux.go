//go:build linux && !android

package ccidusb

import (
	"context"
	"fmt"
	"strings"

	ccid "github.com/creamlike1024/go-ccid"
	"github.com/creamlike1024/go-ccid/usbfs"
)

func platformSource() Source { return usbfsSource{} }

// usbfsSource reaches the readers of usbfs, which the user needs to be
// allowed to open, and which pcscd must not hold.
type usbfsSource struct{}

func (usbfsSource) List(ctx context.Context) ([]Reader, error) {
	infos, err := usbfs.Enumerate()
	if err != nil {
		return nil, err
	}
	readers := make([]Reader, 0, len(infos))
	for _, i := range infos {
		name := strings.TrimSpace(i.Manufacturer + " " + i.Product)
		if name == "" {
			name = fmt.Sprintf("%04x:%04x", i.VendorID, i.ProductID)
		}
		readers = append(readers, Reader{ID: i.Path, Label: fmt.Sprintf("%s (USB %d-%d)", name, i.Bus, i.Addr)})
	}
	return readers, nil
}

func (usbfsSource) Open(ctx context.Context, id string) (ccid.Transport, ccid.Descriptor, error) {
	dev, err := usbfs.Open(id, nil)
	if err != nil {
		return nil, ccid.Descriptor{}, err
	}
	ifs := dev.Interfaces()
	if len(ifs) == 0 {
		dev.Close()
		return nil, ccid.Descriptor{}, fmt.Errorf("%s is no CCID reader", id)
	}
	ep, err := dev.Claim(ifs[0])
	if err != nil {
		dev.Close()
		return nil, ccid.Descriptor{}, err
	}
	return deviceTransport{ep, dev}, ifs[0].Descriptor, nil
}

// deviceTransport closes the device with its endpoints.
type deviceTransport struct {
	*usbfs.Endpoints
	dev *usbfs.Device
}

func (t deviceTransport) Close() error {
	err := t.Endpoints.Close()
	if cerr := t.dev.Close(); err == nil {
		err = cerr
	}
	return err
}
