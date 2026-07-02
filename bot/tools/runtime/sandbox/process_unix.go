//go:build !windows

package sandbox

import (
	"context"
	"fmt"
	"nekocode/bot/tools/runtime/toolutil"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func RunHostBash(ctx context.Context, command string, timeout time.Duration) (string, error) {
	return runExec(ctx, timeout, "bash", "-c", command)
}

func runExec(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}

	stop := context.AfterFunc(cmdCtx, func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	defer stop()

	output, err := cmd.CombinedOutput()
	cleaned := toolutil.StripAnsi(string(output))
	if len(cleaned) > maxOutputBytes {
		cleaned = cleaned[:maxOutputBytes] + "\n[output truncated]\n"
	}
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %v: %v\nOutput: %s", timeout, err, cleaned)
		}
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, cleaned)
	}
	return cleaned, nil
}
