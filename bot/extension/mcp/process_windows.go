//go:build windows

package mcp

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func killProcessTree(process *os.Process) error {
	// taskkill /T terminates descendants as well as the direct MCP process.
	if err := exec.Command("taskkill", "/PID", strconv.Itoa(process.Pid), "/T", "/F").Run(); err == nil {
		return nil
	}
	return process.Kill()
}
