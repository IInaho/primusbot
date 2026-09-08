//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package mcp

import (
	"os"
	"os/exec"
)

func configureProcess(_ *exec.Cmd) {}

func killProcessTree(process *os.Process) error {
	return process.Kill()
}
