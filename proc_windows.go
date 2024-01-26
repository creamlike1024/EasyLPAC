//go:build windows

package main

import (
	"fyne.io/fyne/v2"
	"os/exec"
	"syscall"
)

// HideCmdWindow For windows hide console window
func HideCmdWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}

func SetFixedWindowSize(w *fyne.Window) {}
