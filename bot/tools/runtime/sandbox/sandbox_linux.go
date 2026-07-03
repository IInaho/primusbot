//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"time"
)

// RunSandboxed (Linux): prefers the native namespace backend (pivot_root +
// network/pid isolation). If user namespaces are unavailable (disabled by
// env or rejected by the kernel/container), falls back to the Landlock
// backend (file-write protection only, no network isolation).
//
// The returned error is an UnavailableError when no backend could be used;
// callers should treat it as a signal to request host-execution permission.
func RunSandboxed(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	// Validate the workspace up front so callers always get a clear
	// "workspace is required" error rather than a confusing backend-specific
	// failure (or an UnavailableError from a doomed landlock attempt).
	if profile.Workspace == "" {
		return "", fmt.Errorf("sandbox workspace is required")
	}

	// Fast path: skip the native attempt entirely when it is known to be
	// unavailable (NEKOCODE_DISABLE_NATIVE_SANDBOX set, or unshare probe
	// fails — common on CI/restricted kernels). Avoids spawning a doomed
	// child just to classify its failure.
	if !IsNativeAvailable() {
		if LandlockAvailable() {
			if out, err := RunLandlockBash(ctx, command, profile, timeout); err == nil {
				return out, nil
			}
		}
		return "", UnavailableError{Reason: "native sandbox unavailable and landlock failed or unavailable"}
	}

	out, err := RunNativeBash(ctx, command, profile, timeout)
	if err == nil {
		return out, nil
	}
	// Namespace creation can fail at runtime even when the probe passed
	// (e.g. mount denied mid-setup). Fall back to the Landlock backend.
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
