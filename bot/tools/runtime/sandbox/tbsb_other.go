//go:build !linux

package sandbox

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"nekocode/bot/tools/runtime/toolutil"

	tbsb "github.com/tirdyhouse/sandbox"
)

// RunTbsbBash runs command under the tirdyhouse/sandbox backend (macOS
// sandbox-exec / Windows Low Integrity Level).
//
// It uses the package's public Command API, which applies the platform's
// built-in sandbox so the command can only write to WritableDirs (workspace
// plus allowed cache paths); everything else is read-only.
//
// NOTE: the public Cmd does not expose the child process handle, so per-run
// timeout enforcement is not applied on these platforms. Long-running
// commands rely on the caller's context cancellation. Native timeout/kill
// is available on Linux via the namespace and Landlock backends.
func RunTbsbBash(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	if profile.Workspace == "" {
		return "", fmt.Errorf("sandbox workspace is required")
	}
	ws, err := filepath.Abs(profile.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	if !tbsb.Available() {
		return "", UnavailableError{Reason: tbsb.ReasonUnavailable()}
	}

	writable := []string{ws}
	cachePaths, err := allowedCachePaths(profile.CachePaths)
	if err != nil {
		return "", err
	}
	writable = append(writable, cachePaths...)

	cmd := tbsb.Command("bash", "-c", command)
	cmd.Dir = ws
	cmd.Policy.WritableDirs = writable

	out, err := cmd.CombinedOutput()
	cleaned := toolutil.StripAnsi(string(out))
	if len(cleaned) > maxOutputBytes {
		cleaned = cleaned[:maxOutputBytes] + "\n[output truncated]\n"
	}
	if err != nil {
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, cleaned)
	}
	return cleaned, nil
}
