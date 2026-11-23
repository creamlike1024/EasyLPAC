//go:build !android && !phone

package main

// phoneUI is set where pages take the layout of phones.
const phoneUI = false

// phoneState is the phone frame's; desktops have none.
type phoneState struct{}

// frameChrome is what the desktop's frame takes from a page.
const frameChrome = desktopChrome
