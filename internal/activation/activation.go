// Package activation parses eSIM activation codes (SGP.22 §4.1) and
// decodes them from QR code images. It has no UI dependency.
package activation

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"EasyLPAC/internal/lpa"
)

// ErrInvalidFormat reports an activation code that is not "LPA:1$..."
// with a non-empty SM-DP+ address.
var ErrInvalidFormat = errors.New("activation: invalid activation code format")

// Decode parses an activation code into the download parameters and
// whether a confirmation code is required.
// ref: SGP.22 v2.2.2 §4.1 (Activation Code Format)
func Decode(code string) (info lpa.PullInfo, confirmCodeNeeded bool, err error) {
	err = ErrInvalidFormat
	code = strings.TrimSpace(code)
	var ok bool
	if code, ok = strings.CutPrefix(code, "LPA:"); !ok {
		return
	}
	switch parts := strings.Split(code, "$"); parts[0] {
	case "1":
		var codeNeeded string
		bindings := []*string{&info.SMDP, &info.MatchID, &info.ObjectID, &codeNeeded}
		for index, value := range parts[1:] {
			if index >= len(bindings) {
				break
			}
			*bindings[index] = strings.TrimSpace(value)
		}
		confirmCodeNeeded = codeNeeded == "1"
		if info.SMDP != "" {
			err = nil
		}
	}
	return
}

// Complete adds the "LPA:1" prefix to shortened activation codes
// ("1$addr$id" or "$addr$id"); other input is returned unchanged.
func Complete(input string) string {
	if strings.HasPrefix(input, "LPA:1$") {
		return input
	}
	if strings.HasPrefix(input, "1$") {
		return "LPA:" + input
	}
	if strings.HasPrefix(input, "$") {
		return "LPA:1" + input
	}
	return input
}

// ScanQRCode decodes the text of a QR code in a JPEG or PNG image.
func ScanQRCode(r io.Reader) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}
	return ScanQRCodeImage(img)
}

// ScanQRCodeBytes decodes the text of a QR code in encoded image bytes.
func ScanQRCodeBytes(b []byte) (string, error) {
	return ScanQRCode(bytes.NewReader(b))
}

// ScanQRCodeImage decodes the text of a QR code in a decoded image.
func ScanQRCodeImage(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	result, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		return "", err
	}
	return result.GetText(), nil
}
