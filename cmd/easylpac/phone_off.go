//go:build !android && !phone

package main

import "github.com/egoist/mygo/ui"

// phoneUI is set where pages take the layout of phones.
const phoneUI = false

// phoneState is the phone frame's; desktops have none.
type phoneState struct{}

// frameChrome is what the desktop's frame takes from a page.
const frameChrome = desktopChrome

// phoneQRButtons are the phones' ways to a QR code; desktops open images
// and take them dropped.
func (g *gui) phoneQRButtons(*ui.Context) {}
