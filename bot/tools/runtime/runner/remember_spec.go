package runner

import (
	"path/filepath"
	"strings"
)

// bashRememberSpec derives a command-prefix specifier to remember when the
// user approves a bash command with "remember". "npm run build" → "npm run *"
// (broaden to the first two words so similar commands auto-approve). Single-
// word commands stay exact.
func bashRememberSpec(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	fields := strings.Fields(cmd)
	switch {
	case len(fields) == 0:
		return ""
	case len(fields) == 1:
		return fields[0]
	default:
		return fields[0] + " " + fields[1] + " *"
	}
}

// pathRememberSpec derives a path specifier to remember for file tools.
// Workspace-relative paths stay relative ("/src/foo.go" → "/src/**" is too
// broad; instead remember the file's parent dir: "/src/**"). Home paths get
// "~/" prefix; absolute paths get "//".
func pathRememberSpec(p, workspace, home string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rel := ""
	if workspace != "" {
		if r, err := filepath.Rel(workspace, abs); err == nil && !strings.HasPrefix(r, "..") {
			rel = "/" + filepath.ToSlash(r)
		}
	}
	if rel != "" {
		// remember the parent directory as a writable tree
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "/." || dir == "/" {
			return "/" // whole workspace
		}
		return dir + "/**"
	}
	if home != "" && strings.HasPrefix(abs, home+"/") {
		return "~/" + filepath.ToSlash(strings.TrimPrefix(abs, home+"/"))
	}
	return "//" + filepath.ToSlash(abs)
}
