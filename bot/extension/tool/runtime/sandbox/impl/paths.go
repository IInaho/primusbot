package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxOutputBytes caps captured output across all backends.
const maxOutputBytes = 1024 * 1024
const captureHeadLines = 50
const captureTailLines = 50

func truncateCapturedOutput(output string) string {
	if len(output) <= maxOutputBytes {
		return output
	}
	body := strings.TrimRight(output, "\n")
	lines := strings.Split(body, "\n")
	if len(lines) > captureHeadLines+captureTailLines {
		head := strings.Join(lines[:captureHeadLines], "\n")
		tail := strings.Join(lines[len(lines)-captureTailLines:], "\n")
		return fmt.Sprintf("%s\n[... %d lines truncated, %d bytes total ...]\n%s\n",
			head, len(lines)-captureHeadLines-captureTailLines, len(output), tail)
	}

	edgeBytes := maxOutputBytes / 4
	if edgeBytes < 1 {
		edgeBytes = 1
	}
	return fmt.Sprintf("%s\n[... output truncated, %d bytes total ...]\n%s",
		output[:edgeBytes], len(output), output[len(output)-edgeBytes:])
}

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
