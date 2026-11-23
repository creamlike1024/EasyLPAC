package activation

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// photo is a camera's photo, w by h, of a QR code of text whose side is
// side pixels, on a gray background, off its center.
func photo(t *testing.T, text string, w, h, side int) image.Image {
	t.Helper()
	m, err := qrcode.NewQRCodeWriter().Encode(text, gozxing.BarcodeFormat_QR_CODE, side, side, nil)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0xB0, 0xA8, 0xA0, 0xFF}}, image.Point{}, draw.Src)
	ox, oy := w/3, h/4
	for y := range m.GetHeight() {
		for x := range m.GetWidth() {
			c := color.RGBA{0xF4, 0xF4, 0xF0, 0xFF}
			if m.Get(x, y) {
				c = color.RGBA{0x20, 0x20, 0x28, 0xFF}
			}
			img.Set(ox+x, oy+y, c)
		}
	}
	return img
}

func TestScanPhoto(t *testing.T) {
	const code = "LPA:1$smdp.example.com$04386-AGYFT-A74Y8-3F815"
	for _, tc := range []struct {
		name       string
		w, h, side int
		jpeg       bool
	}{
		{"large code, JPEG", 4000, 3000, 1400, true},
		{"small code, JPEG", 4000, 3000, 500, true},
		{"large code, RGBA", 4000, 3000, 1400, false},
		{"screenshot", 720, 1440, 400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := photo(t, code, tc.w, tc.h, tc.side)
			if tc.jpeg {
				var b bytes.Buffer
				if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 85}); err != nil {
					t.Fatal(err)
				}
				var err error
				if img, _, err = image.Decode(&b); err != nil {
					t.Fatal(err)
				}
				if _, ok := img.(*image.YCbCr); !ok {
					t.Fatalf("JPEG decoded as %T", img)
				}
			}
			got, err := ScanQRCodeImage(img)
			if err != nil || got != code {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
