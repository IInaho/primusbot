//go:build linux

package sandbox

import (
	"context"
	"time"
)

// RunSandboxed (Linux): prefers the native namespace backend (pivot_root +
// network/pid isolation). If user namespaces are unavailable, falls back to
// the Landlock backend (file-write protection only, no network isolation).
//
// The returned error is an UnavailableError when no backend could be used;
// callers should treat it as a signal to request host-execution permission.
func RunSandboxed(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	out, err := RunNativeBash(ctx, command, profile, timeout)
	if err == nil {
		return out, nil
	}
	// Namespace creation can fail on kernels/container runtimes that disable
	// user namespaces. Fall back to the Landlock backend.
	if unavail, ok := err.(UnavailableError); ok {
		if LandlockAvailable() {
			if lbOut, lbErr := RunLandlockBash(ctx, command, profile, timeout); lbErr == nil {
				return lbOut, nil
			}
		}
		return "", unavail
	}
	return out, err
}
