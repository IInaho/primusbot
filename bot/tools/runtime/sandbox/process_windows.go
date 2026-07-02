//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"nekocode/bot/tools/runtime/toolutil"
)

// RunHostBash runs command on the host without sandboxing (Windows).
// Process-group kill is not applied; only context cancellation is honoured.
func RunHostBash(ctx context.Context, command string, timeout time.Duration) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "bash", "-c", command).CombinedOutput()
	cleaned := toolutil.StripAnsi(string(out))
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
