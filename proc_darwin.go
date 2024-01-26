//go:build darwin

package main

import (
	"fyne.io/fyne/v2"
	"os/exec"
)

func HideCmdWindow(cmd *exec.Cmd) {}

func SetFixedWindowSize(w *fyne.Window) {
	(*w).SetFixedSize(true)
}
