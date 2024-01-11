//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// HideCmdWindow For windows hide console window
func HideCmdWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}
