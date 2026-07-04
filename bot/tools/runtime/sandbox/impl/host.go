package impl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"nekocode/bot/tools/runtime/toolutil"
)

// RunHost executes command on the host with NO sandbox isolation.
// Use only when the caller has an explicit process.host grant.
func RunHost(ctx context.Context, command string, timeout time.Duration) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}

	if runtime.GOOS != "windows" {
		// Kill the whole process group on timeout/cancel so nested children
		// (e.g. `bash -c 'sleep 30'`) cannot outlive the deadline.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stop := context.AfterFunc(cmdCtx, func() {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		})
		defer stop()
	}

	output, err := cmd.CombinedOutput()
	cleaned := toolutil.StripAnsi(string(output))
	cleaned = truncateCapturedOutput(cleaned)
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %v: %v\nOutput: %s", timeout, err, cleaned)
		}
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, cleaned)
	}
	return cleaned, nil
}
