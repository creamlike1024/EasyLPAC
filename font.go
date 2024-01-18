package main

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed assets/NotoSansMonoCJKsc-Regular.otf
var monoFontRegular []byte

//go:embed assets/DejaVuSansMono.ttf
var dejaVuMonoFontRegular []byte

var resourceNotoSansMonoCJKscRegular = &fyne.StaticResource{
	StaticName:    "NotoSansMonoCJKsc-Regular.otf",
	StaticContent: monoFontRegular,
}

var resourceDejaVuSansMonoRegular = &fyne.StaticResource{
	StaticName:    "DejaVuSansMono.ttf",
	StaticContent: dejaVuMonoFontRegular,
}
