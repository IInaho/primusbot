//go:build !linux

package sandbox

import (
	"context"
	"time"
)

// RunSandboxed (macOS/Windows): uses the github.com/tirdyhouse/sandbox backend
// (macOS sandbox-exec / Windows Low Integrity Level) for file-write protection.
//
// The returned error is an UnavailableError when the platform backend is not
// available; callers should treat it as a signal to request host-execution
// permission.
func RunSandboxed(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	return RunTbsbBash(ctx, command, profile, timeout)
}
