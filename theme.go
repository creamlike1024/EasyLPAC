package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type myTheme struct{}

func (myTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(n, v)
}

func (myTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
		return resourceNotoSansMonoCJKscRegular
	}
	if s.Italic {
		return resourceDejaVuSansMonoRegular
	}
	return theme.DefaultTheme().Font(s)
}

func (myTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (myTheme) Size(s fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(s)
}
