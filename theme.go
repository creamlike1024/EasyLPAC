package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type MyTheme struct{}

func (MyTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0xe6, G: 0x77, B: 0x2e, A: 0xff}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0xe6, G: 0x77, B: 0x2e, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xf5, G: 0x65, B: 0x08, A: 0x2a}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xf5, G: 0x65, B: 0x08, A: 0x2a}
	default:
		return theme.DefaultTheme().Color(n, v)
	}
}

func (MyTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Italic || s.Symbol {
		return theme.DefaultTheme().Font(s)
	}
	if s.Monospace {
		return resourceDroidSansMono
	}
	if s.Bold {
		switch LanguageTag {
			case "ja-JP":
				return resourceNotoSansJPBold
			case "zh-TW":
				return resourceNotoSansTCBold
			default:
				return resourceDroidSansBold
		}
	}
	switch LanguageTag {
		case "ja-JP":
			return resourceNotoSansJP
		case "zh-TW":
			return resourceNotoSansTC
		default:
			return resourceDroidSansFallback
	}
}

func (MyTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (MyTheme) Size(s fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(s)
}
