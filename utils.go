package main

import (
	"bytes"
	"errors"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"golang.design/x/clipboard"
	"image"
	_ "image/jpeg"
	"os"
	"strings"
)

func CountryCodeToEmoji(countryCode string) string {
	if len(countryCode) != 2 {
		return "🌎"
	}
	countryCode = strings.ToUpper(countryCode)
	rune1 := rune(countryCode[0]-'A') + 0x1F1E6
	rune2 := rune(countryCode[1]-'A') + 0x1F1E6
	return string([]rune{rune1, rune2})
}

func DecodeLPADownloadConfig(s string) (PullInfo, error) {
	strs := strings.Split(s, "$")
	if len(strs) != 3 {
		return PullInfo{}, errors.New("QR code or LPA string format error")
	}
	if strings.TrimSpace(strs[0]) != "LPA:1" {
		return PullInfo{}, errors.New("QR Code or LPA string format error")
	}
	return PullInfo{
		SMDP:        strs[1],
		MatchID:     strs[2],
		ConfirmCode: "",
		IMEI:        "",
	}, nil
}

func scanQRCodeFromImage(img image.Image) (*gozxing.Result, error) {
	// prepare BinaryBitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return &gozxing.Result{}, err
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return &gozxing.Result{}, err
	}
	return result, nil
}

func ScanQRCodeImageFile(filename string) (*gozxing.Result, error) {
	// open and decode image file
	file, err := os.Open(filename)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	if err != nil {
		return &gozxing.Result{}, err
	}
	img, _, err := image.Decode(file)
	if err != nil {
		return &gozxing.Result{}, err
	}

	return scanQRCodeFromImage(img)
}

func ScanQRCodeImageBytes(imageBytes []byte) (*gozxing.Result, error) {
	// Decode image bytes
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return &gozxing.Result{}, err
	}

	return scanQRCodeFromImage(img)
}

func PasteFromClipboard() (clipboard.Format, []byte, error) {
	// It seems no wayland support now
	// Clipboard API provided by fyne does not meet the requirements since it only support string
	// So I introduced 3rd party clipboard lib `golang.design/x/clipboard`
	// ref: https://docs.fyne.io/api/v2.4/clipboard.html
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}
	result := clipboard.Read(clipboard.FmtText)
	if len(result) != 0 {
		return clipboard.FmtText, result, nil
	}
	result = clipboard.Read(clipboard.FmtImage)
	if len(result) != 0 {
		return clipboard.FmtImage, result, nil
	}
	return clipboard.FmtText, nil, errors.New("failed to get content from clipboard")
}
