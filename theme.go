package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type myTheme struct{}

func (myTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0xf6, G: 0x5d, B: 0x29, A: 0xff}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0xf6, G: 0x5d, B: 0x29, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xf5, G: 0x65, B: 0x08, A: 0x2a}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xf5, G: 0x65, B: 0x08, A: 0x2a}
	default:
		return theme.DefaultTheme().Color(n, v)
	}
}

func (myTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
		return resourceNotoSansMonoCJKscRegular
	}
	return theme.DefaultTheme().Font(s)
}

func (myTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (myTheme) Size(s fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(s)
}
