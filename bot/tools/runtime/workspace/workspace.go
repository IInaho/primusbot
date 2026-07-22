package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Access string

const (
	AccessReadOnly  Access = "read-only"
	AccessReadWrite Access = "read-write"
)

type Root struct {
	Path   string `json:"path"`
	Access Access `json:"access"`
}

var current = struct {
	sync.RWMutex
	// configured roots come from Configure (cwd + config file + remembered
	// roots) and are replaced wholesale on every reload.
	configured []Root
	// session roots come from AddSessionRoot (user approved a workspace
	// during the session) and survive Configure reloads.
	session []Root
}{}

func Configure(primary string, extra []Root) {
	roots := []Root{{Path: primary, Access: AccessReadWrite}}
	roots = append(roots, extra...)
	current.Lock()
	current.configured = normalizeRoots(roots)
	current.Unlock()
}

func AddSessionRoot(path string, access Access) (Root, error) {
	root, err := normalizeRoot(Root{Path: path, Access: access})
	if err != nil {
		return Root{}, err
	}
	current.Lock()
	defer current.Unlock()
	current.session = upsertRoot(current.session, root)
	return root, nil
}

func Snapshot() []Root {
	current.RLock()
	roots := append([]Root(nil), current.configured...)
	for _, r := range current.session {
		roots = upsertRoot(roots, r)
	}
	current.RUnlock()
	if len(roots) > 0 {
		return roots
	}
	return fallbackRoots()
}

func CheckRead(path string) (string, Root, bool, error) {
	return check(path, AccessReadOnly)
}

func CheckWrite(path string) (string, Root, bool, error) {
	return check(path, AccessReadWrite)
}

func check(path string, need Access) (string, Root, bool, error) {
	resolved, err := Resolve(path)
	if err != nil {
		return "", Root{}, false, err
	}
	for _, root := range Snapshot() {
		if !rootAllows(root, need) {
			continue
		}
		if insideRoot(resolved, root.Path) {
			return resolved, root, true, nil
		}
	}
	return resolved, Root{}, false, nil
}

func Resolve(path string) (string, error) {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		parent := filepath.Dir(abs)
		realParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			return filepath.Clean(abs), nil
		}
		return filepath.Join(realParent, filepath.Base(abs)), nil
	}
	return real, nil
}

func CandidateRoot(path string) (string, error) {
	resolved, err := Resolve(path)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved, nil
	}
	return filepath.Dir(resolved), nil
}

func normalizeRoots(roots []Root) []Root {
	out := make([]Root, 0, len(roots))
	for _, r := range roots {
		root, err := normalizeRoot(r)
		if err != nil {
			continue
		}
		out = upsertRoot(out, root)
	}
	slices.SortFunc(out, func(a, b Root) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func normalizeRoot(r Root) (Root, error) {
	p := strings.TrimSpace(r.Path)
	if p == "" {
		return Root{}, fmt.Errorf("workspace path is required")
	}
	p = expandHome(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return Root{}, err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	r.Path = filepath.Clean(abs)
	if r.Access != AccessReadOnly && r.Access != AccessReadWrite {
		r.Access = AccessReadOnly
	}
	return r, nil
}

// expandHome resolves a leading ~/ to the user's home directory so paths
// handed to Resolve and root normalization behave consistently.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, _ := os.UserHomeDir(); home != "" {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}

func upsertRoot(roots []Root, root Root) []Root {
	for i, existing := range roots {
		if existing.Path != root.Path {
			continue
		}
		if rootAllows(root, AccessReadWrite) || !rootAllows(existing, AccessReadWrite) {
			roots[i] = root
		}
		return roots
	}
	return append(roots, root)
}

func rootAllows(root Root, need Access) bool {
	if need == AccessReadOnly {
		return root.Access == AccessReadOnly || root.Access == AccessReadWrite
	}
	return root.Access == AccessReadWrite
}

func insideRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func fallbackRoots() []Root {
	cwd := os.Getenv("NEKOCODE_WORKSPACE")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	roots := []Root{{Path: cwd, Access: AccessReadWrite}}
	if extra := os.Getenv("NEKOCODE_EXTRA_DIRS"); extra != "" {
		for _, p := range filepath.SplitList(extra) {
			roots = append(roots, Root{Path: p, Access: AccessReadWrite})
		}
	}
	return normalizeRoots(roots)
}
