//go:build !windows

package main

import (
	"os/exec"
)

func HideCmdWindow(cmd *exec.Cmd) {
	// Do nothing on non-Windows systems.
}
