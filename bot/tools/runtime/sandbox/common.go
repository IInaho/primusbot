package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxOutputBytes caps captured output across all backends.
const maxOutputBytes = 1024 * 1024

// UnavailableError is returned when no sandbox backend could be used.
// Callers should treat it as a signal to request host-execution permission.
type UnavailableError struct {
	Reason string
}

func (e UnavailableError) Error() string {
	if e.Reason == "" {
		return "sandbox unavailable"
	}
	return "sandbox unavailable: " + e.Reason
}

// BashProfile describes the sandbox environment for a command.
type BashProfile struct {
	Workspace  string
	Network    bool
	CachePaths []string
	// StagingRoot is a host-side mountpoint (created and cleaned up by the
	// parent) where the child mounts a tmpfs that becomes the sandbox root.
	// Empty means the child picks a default (then no cleanup is performed).
	StagingRoot string
}

// allowedCachePaths filters and resolves cache paths, rejecting any
// that fall outside the allowed roots.
func allowedCachePaths(paths []string) ([]string, error) {
	home, _ := os.UserHomeDir()
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		if strings.HasPrefix(p, "~/") && home != "" {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
		p = filepath.Clean(p)
		if p == "." || seen[p] {
			continue
		}
		if !isAllowedCachePath(home, p) {
			return nil, fmt.Errorf("cache path is outside allowed cache roots: %s", p)
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}

func isAllowedCachePath(home, p string) bool {
	if home == "" {
		return false
	}
	allowed := []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".pnpm-store"),
		filepath.Join(home, ".cache", "yarn"),
		filepath.Join(home, "go", "pkg", "mod"),
		filepath.Join(home, ".cargo"),
	}
	for _, root := range allowed {
		if p == root || strings.HasPrefix(p, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
