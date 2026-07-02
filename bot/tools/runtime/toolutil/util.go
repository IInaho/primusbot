package toolutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/editcore"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/common"
)

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func ValidatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		parent := filepath.Dir(abs)
		realParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			return abs, nil
		}
		return filepath.Join(realParent, filepath.Base(abs)), nil
	}
	return real, nil
}

// ValidatePathInWorkspace resolves path and enforces that it lives inside the
// workspace (process cwd) or one of the extra allowed roots
// (NEKOCODE_EXTRA_DIRS, colon-separated). Writes via the write/edit tools are
// confined to these roots; paths outside are rejected so the LLM cannot
// touch /etc, ~/.ssh, etc. without going through bash (which has its own
// OS sandbox). Read-only tools (read/glob/grep) use ValidatePath instead and
// stay allowed everywhere, gated by permission deny rules for sensitive files.
func ValidatePathInWorkspace(path string) (string, error) {
	abs, err := ValidatePath(path)
	if err != nil {
		return "", err
	}
	if isInsideAnyRoot(abs, allowedWriteRoots()) {
		return abs, nil
	}
	return "", fmt.Errorf("path %s is outside the workspace (allowed roots: %v); use bash for writes outside the workspace", abs, allowedWriteRoots())
}

func allowedWriteRoots() []string {
	// Workspace is injected via env (set by the Bot at startup so write/edit
	// tools resolve against the session workspace, not the package build dir).
	// Falls back to the process cwd for ad-hoc usage / tests that chdir.
	cwd := os.Getenv("NEKOCODE_WORKSPACE")
	if cwd == "" {
		if d, err := os.Getwd(); err == nil && d != "" {
			cwd = d
		} else {
			cwd = "."
		}
	}
	roots := []string{filepath.Clean(cwd)}
	if extra := os.Getenv("NEKOCODE_EXTRA_DIRS"); extra != "" {
		for _, r := range filepath.SplitList(extra) {
			if r = filepath.Clean(r); r != "" {
				roots = append(roots, r)
			}
		}
	}
	return roots
}

func isInsideAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func NormalizeText(text string) string {
	text = StripAnsi(text)
	return editcore.NormalizeToLF(text)
}

func ReadSafeFile(path string) ([]byte, error) {
	safePath, err := ValidatePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(safePath)
}

func NewToolHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: common.SharedTransport,
		Timeout:   timeout,
	}
}

func IsAllExploratory(calls []core.ToolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		switch c.Name {
		case "read", "grep", "glob", "list", "web_search", "web_fetch":
			continue
		default:
			return false
		}
	}
	return true
}

func RecordSnapshot(path, content string) string {
	return recordSnapshot(execution.GetGlobalSnapshotStore(), path, content)
}

func RecordSnapshotInContext(ctx context.Context, path, content string) string {
	return recordSnapshot(execution.SnapshotStoreFromContext(ctx), path, content)
}

func recordSnapshot(store *editcore.SnapshotStore, path, content string) string {
	if store == nil {
		return ""
	}
	return store.Record(path, content)
}
