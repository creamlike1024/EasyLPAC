// Package euicc binds the vendored static libeuicc (LGPL-2.1, built per
// target by scripts/build-libeuicc.sh into
// third_party/libeuicc/<GOOS>-<GOARCH>; see
// third_party/lpac/euicc/LICENSE) to Go via an original C shim with
// opaque handles. Memory, callback and lifecycle contracts are defined
// in docs/refactor-architecture.md §4.
package euicc

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/lpac
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/darwin-arm64
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/darwin-amd64
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/linux-amd64
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/linux-arm64
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/windows-amd64
#cgo windows,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/libeuicc/windows-arm64
#cgo LDFLAGS: -leuicc -lcjsonext -lcjson

#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// libeuicc has no version of its own: it is built from the lpac source
// tree, so the lpac commit it comes from is its version, written as the
// short commit (LpacShortCommit). LpacCommit must name the commit the
// third_party/lpac submodule points at (checked by a test); bump both
// with the submodule.
const LpacCommit = "654215e44b22dbbd1377932b2fa4c76d308cc5ab"

// LpacShortCommit is libeuicc's version: the short lpac commit.
const LpacShortCommit = "654215e"

// APDUInterface is implemented by transport backends (pcsc, at, ...).
// Transmit receives a complete raw APDU and must return the raw response
// including SW1/SW2.
type APDUInterface interface {
	Connect() error
	Disconnect()
	LogicChannelOpen(aid []byte) (channel uint8, err error)
	LogicChannelClose(channel uint8)
	Transmit(apdu []byte) (response []byte, err error)
}

// HTTPInterface is implemented by the Go net/http adapter.
// tx is the request body; headers is a NULL-terminated style list in Go
// slice form (even length: key,value pairs as provided by libeuicc).
// The return value is (httpStatusCode, responseBody, error).
type HTTPInterface interface {
	Transmit(url string, headers []string, tx []byte) (rcode uint32, rx []byte, err error)
}

// Context is one libeuicc context. Not safe for concurrent use.
type Context struct {
	h     *C.elpac_ctx
	apduH cgo.Handle
	httpH cgo.Handle

	apdu        APDUInterface
	http        HTTPInterface
	initialized bool
	// lastErr is the last error a callback returned to libeuicc, which
	// only sees -1: failed adds it to the error of the call that failed.
	lastErr error
}

// failed reports that libeuicc's fn failed, with the reason a callback
// gave (a transport or HTTP error) when there is one.
func (c *Context) failed(fn string) error {
	err := c.lastErr
	c.lastErr = nil
	if err != nil {
		return fmt.Errorf("euicc: %s failed: %w", fn, err)
	}
	return fmt.Errorf("euicc: %s failed", fn)
}

// recordingAPDU and recordingHTTP keep the errors the callbacks return.
type recordingAPDU struct {
	APDUInterface
	c *Context
}

func (r recordingAPDU) Connect() error { return r.c.keep(r.APDUInterface.Connect()) }

func (r recordingAPDU) LogicChannelOpen(aid []byte) (uint8, error) {
	ch, err := r.APDUInterface.LogicChannelOpen(aid)
	return ch, r.c.keep(err)
}

func (r recordingAPDU) Transmit(apdu []byte) ([]byte, error) {
	resp, err := r.APDUInterface.Transmit(apdu)
	return resp, r.c.keep(err)
}

type recordingHTTP struct {
	HTTPInterface
	c *Context
}

func (r recordingHTTP) Transmit(url string, headers []string, tx []byte) (uint32, []byte, error) {
	code, rx, err := r.HTTPInterface.Transmit(url, headers, tx)
	return code, rx, r.c.keep(err)
}

func (c *Context) keep(err error) error {
	if err != nil {
		c.lastErr = err
	}
	return err
}

// New creates a context bound to the given interfaces. aid is copied.
// The caller must call Close when done.
func New(apdu APDUInterface, httpi HTTPInterface, aid []byte) (*Context, error) {
	if len(aid) == 0 {
		return nil, errors.New("euicc: empty AID")
	}
	c := &Context{apdu: apdu, http: httpi}
	c.apduH = cgo.NewHandle(APDUInterface(recordingAPDU{apdu, c}))
	c.httpH = cgo.NewHandle(HTTPInterface(recordingHTTP{httpi, c}))

	aidPtr := C.malloc(C.size_t(len(aid)))
	if aidPtr == nil {
		c.apduH.Delete()
		c.httpH.Delete()
		return nil, errors.New("euicc: aid allocation failed")
	}
	copy(unsafe.Slice((*byte)(aidPtr), len(aid)), aid)

	c.h = C.elpac_new((*C.uint8_t)(aidPtr), C.uint8_t(len(aid)))
	if c.h == nil {
		C.free(aidPtr)
		c.apduH.Delete()
		c.httpH.Delete()
		return nil, errors.New("euicc: context allocation failed")
	}

	if rc := C.elpac_set_interfaces(c.h, C.uintptr_t(c.apduH), C.uintptr_t(c.httpH)); rc != 0 {
		C.elpac_free(c.h)
		C.free(aidPtr)
		c.apduH.Delete()
		c.httpH.Delete()
		return nil, fmt.Errorf("euicc: interface setup failed (%d)", int(rc))
	}

	if rc := C.elpac_init(c.h); rc != 0 {
		C.elpac_free(c.h)
		C.free(aidPtr)
		err := c.failed("euicc_init")
		c.apduH.Delete()
		c.httpH.Delete()
		return nil, fmt.Errorf("%w (%d)", err, int(rc))
	}
	c.initialized = true
	return c, nil
}

// SetDebug toggles APDU/HTTP debug logging to the standard log writer.
func (c *Context) SetDebug(apdu, httpDebug bool) {
	C.elpac_set_debug(c.h, boolToC(apdu), boolToC(httpDebug))
}

func boolToC(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// Close releases the context. Order per contract: stop callbacks from
// reaching Go (shim clears interface pointers first), free C resources,
// then delete the cgo handles. Idempotent.
func (c *Context) Close() {
	if c.h == nil {
		return
	}
	if c.initialized {
		C.elpac_fini(c.h)
	}
	C.elpac_free(c.h) // also frees the AID copy and cleans http state
	c.h = nil
	c.apduH.Delete()
	c.httpH.Delete()
}

func apduFromHandle(h C.uintptr_t) APDUInterface {
	return cgo.Handle(h).Value().(APDUInterface)
}

// GetEID returns the EID as an uppercase hex string.
func (c *Context) GetEID() (string, error) {
	var out *C.char
	if rc := C.elpac_get_eid(c.h, &out); rc != 0 {
		return "", c.failed("es10c_get_eid")
	}
	defer C.elpac_string_free(out)
	return C.GoString(out), nil
}

func httpFromHandle(h C.uintptr_t) HTTPInterface {
	return cgo.Handle(h).Value().(HTTPInterface)
}
