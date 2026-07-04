package impl

import (
	"os"
	"path/filepath"
	"strings"
)

// maxOutputBytes caps captured output across all backends.
const maxOutputBytes = 1024 * 1024

// resolveWritePaths cleans, resolves ~/ prefixes, and deduplicates write
// paths. It does NOT enforce a whitelist — authorization is the caller's
// responsibility.
func resolveWritePaths(paths []string) ([]string, error) {
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
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}
