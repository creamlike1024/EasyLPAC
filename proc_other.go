//go:build !windows && !darwin

package main

import (
	"fyne.io/fyne/v2"
	"os/exec"
)

func HideCmdWindow(cmd *exec.Cmd) {
	// Do nothing on non-Windows systems.
}

func SetFixedWindowSize(w *fyne.Window) {}
