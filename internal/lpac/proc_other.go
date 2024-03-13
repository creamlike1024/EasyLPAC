//go:build !windows

package lpac

import (
	"os/exec"
)

func hideWindow(_ *exec.Cmd) {
	// Do nothing on non-Windows systems.
}
