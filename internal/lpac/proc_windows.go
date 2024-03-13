//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow for windows hide console window
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
