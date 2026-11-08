package euicc

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Allocation note: buffers returned to libeuicc from APDU/HTTP
// transmit callbacks must be C.malloc'd; libeuicc frees them.

func copyToCBuffer(data []byte) (*C.uint8_t, C.uint32_t, error) {
	if len(data) == 0 {
		return nil, 0, nil
	}
	if len(data) > int(^C.uint32_t(0)) {
		return nil, 0, errors.New("euicc: response exceeds uint32 length")
	}
	buf := C.malloc(C.size_t(len(data)))
	if buf == nil {
		return nil, 0, errors.New("euicc: response allocation failed")
	}
	copy(unsafe.Slice((*byte)(buf), len(data)), data)
	return (*C.uint8_t)(buf), C.uint32_t(len(data)), nil
}

//export elpacGoApduConnect
func elpacGoApduConnect(token C.uintptr_t) C.int {
	if err := apduFromHandle(token).Connect(); err != nil {
		return -1
	}
	return 0
}

//export elpacGoApduDisconnect
func elpacGoApduDisconnect(token C.uintptr_t) {
	apduFromHandle(token).Disconnect()
}

//export elpacGoApduLogicChannelOpen
func elpacGoApduLogicChannelOpen(token C.uintptr_t, aid *C.uint8_t, aidLen C.uint8_t) C.int {
	aidGo := C.GoBytes(unsafe.Pointer(aid), C.int(aidLen))
	channel, err := apduFromHandle(token).LogicChannelOpen(aidGo)
	if err != nil {
		return -1
	}
	return C.int(channel)
}

//export elpacGoApduLogicChannelClose
func elpacGoApduLogicChannelClose(token C.uintptr_t, channel C.uint8_t) {
	apduFromHandle(token).LogicChannelClose(uint8(channel))
}

//export elpacGoApduTransmit
func elpacGoApduTransmit(token C.uintptr_t, tx *C.uint8_t, txLen C.uint32_t,
	rx **C.uint8_t, rxLen *C.uint32_t) C.int {
	txGo := C.GoBytes(unsafe.Pointer(tx), C.int(txLen))
	resp, err := apduFromHandle(token).Transmit(txGo)
	if err != nil {
		return -1
	}
	buf, n, err := copyToCBuffer(resp)
	if err != nil {
		return -1
	}
	*rx = buf
	*rxLen = n
	return 0
}

//export elpacGoHttpTransmit
func elpacGoHttpTransmit(token C.uintptr_t, url *C.char, headers **C.char,
	tx *C.uint8_t, txLen C.uint32_t,
	rx **C.uint8_t, rxLen *C.uint32_t, rcode *C.uint32_t) C.int {
	urlGo := C.GoString(url)

	// headers is a NULL-terminated array of strings.
	var headersGo []string
	if headers != nil {
		for p := headers; *p != nil; p = (**C.char)(unsafe.Add(unsafe.Pointer(p), unsafe.Sizeof(*p))) {
			headersGo = append(headersGo, C.GoString(*p))
		}
	}

	var txGo []byte
	if tx != nil && txLen > 0 {
		txGo = C.GoBytes(unsafe.Pointer(tx), C.int(txLen))
	}

	code, resp, err := httpFromHandle(token).Transmit(urlGo, headersGo, txGo)
	if err != nil {
		return -1
	}
	buf, n, err := copyToCBuffer(resp)
	if err != nil {
		return -1
	}
	*rx = buf
	*rxLen = n
	*rcode = C.uint32_t(code)
	return 0
}
