package main

import (
	"errors"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
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
		return PullInfo{}, errors.New("QR code format error")
	}
	if strings.TrimSpace(strs[0]) != "LPA:1" {
		return PullInfo{}, errors.New("QR code format error")
	}
	return PullInfo{
		SMDP:        strs[1],
		MatchID:     strs[2],
		ConfirmCode: "",
		IMEI:        "",
	}, nil
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
