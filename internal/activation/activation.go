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

// ScanQRCodeImage decodes the text of a QR code in a decoded image. A
// photo, as a phone's camera takes, is scaled down first, which is faster
// and finds the code as well; what is not found is looked for again,
// harder, at smaller sizes.
func ScanQRCodeImage(img image.Image) (string, error) {
	var err error
	for i, side := range scanSides(img.Bounds()) {
		var hints map[gozxing.DecodeHintType]interface{}
		if i > 0 {
			hints = map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
		}
		var text string
		if text, err = scanAt(img, side, hints); err == nil {
			return text, nil
		}
	}
	return "", err
}

// maxScanSide is the longest side a code is first looked for at.
const maxScanSide = 1600

// scanSides are the longest sides to look for a code at: the image's own
// up to maxScanSide, then the same and 800, harder.
func scanSides(b image.Rectangle) []int {
	first := min(max(b.Dx(), b.Dy()), maxScanSide)
	if first > 800 {
		return []int{first, first, 800}
	}
	return []int{first, first}
}

func scanAt(img image.Image, side int, hints map[gozxing.DecodeHintType]interface{}) (string, error) {
	if b := img.Bounds(); max(b.Dx(), b.Dy()) > side {
		img = shrink(img, side)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	result, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return "", err
	}
	return result.GetText(), nil
}

// shrink scales img down in gray to a longest side of side, each pixel the
// average of those it covers.
func shrink(img image.Image, side int) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := float64(max(w, h)) / float64(side)
	dw, dh := max(int(float64(w)/scale), 1), max(int(float64(h)/scale), 1)
	out := image.NewGray(image.Rect(0, 0, dw, dh))
	luma := func(x, y int) uint64 {
		r, g, bl, _ := img.At(x, y).RGBA()
		return (19595*uint64(r) + 38470*uint64(g) + 7471*uint64(bl) + 1<<15) >> 24
	}
	if ycc, ok := img.(*image.YCbCr); ok {
		// A JPEG's: its Y is the gray.
		luma = func(x, y int) uint64 { return uint64(ycc.Y[ycc.YOffset(x, y)]) }
	}
	for y := range dh {
		y0, y1 := b.Min.Y+y*h/dh, b.Min.Y+(y+1)*h/dh
		for x := range dw {
			x0, x1 := b.Min.X+x*w/dw, b.Min.X+(x+1)*w/dw
			var sum, n uint64
			for sy := y0; sy < max(y1, y0+1); sy++ {
				for sx := x0; sx < max(x1, x0+1); sx++ {
					sum += luma(sx, sy)
					n++
				}
			}
			out.Pix[y*out.Stride+x] = uint8(sum / n)
		}
	}
	return out
}
