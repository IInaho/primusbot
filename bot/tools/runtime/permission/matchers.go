package permission

import (
	"path/filepath"
	"strings"
)

// BashMatcher matches bash command rules. Specifier semantics (claude-code):
//
//	"npm run *"   matches commands starting with "npm run " (* = any suffix)
//	"npm run"     exact match (command is exactly "npm run")
//	"npm run:*"   same as "npm run *" (trailing :* form)
//	"*"           matches every command
//
// Compound commands (cmd1 && cmd2) are split on &&, ||, ;, |, & and newlines;
// the rule must match EVERY subcommand for the call to match (so an allow
// rule can't be bypassed by chaining with a disallowed command). A bare
// process wrapper (timeout/time/nice/nohup) is stripped before matching.
type BashMatcher struct{}

func (BashMatcher) Match(spec string, info map[string]any) (bool, error) {
	cmd, _ := info["command"].(string)
	if cmd == "" || spec == "" {
		return false, nil
	}
	if spec == "*" {
		return true, nil
	}
	pattern := normalizeBashSpec(spec)
	subcmds := splitCompound(cmd)
	if len(subcmds) == 0 {
		return false, nil
	}
	for _, sub := range subcmds {
		if !matchBashPattern(pattern, sub) {
			return false, nil
		}
	}
	return true, nil
}

// normalizeBashSpec turns "npm run:*" into "npm run *".
func normalizeBashSpec(spec string) string {
	if strings.HasSuffix(spec, ":*") {
		return strings.TrimSuffix(spec, ":*") + " *"
	}
	return spec
}

// matchBashPattern checks whether a single (sub)command matches a pattern.
// A trailing " *" enforces a word boundary (space or end); a trailing "*"
// without space matches any suffix.
func matchBashPattern(pattern, cmd string) bool {
	cmd = strings.TrimSpace(stripWrappers(cmd))
	pattern = strings.TrimSpace(pattern)
	if pattern == cmd {
		return true
	}
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		return cmd == prefix || strings.HasPrefix(cmd, prefix+" ")
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(cmd, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

var bashWrappers = []string{"timeout", "time", "nice", "nohup", "stdbuf"}

// stripWrappers removes a leading process wrapper so "timeout 30 npm test"
// matches a "npm test *" rule.
func stripWrappers(cmd string) string {
	for {
		stripped := false
		for _, w := range bashWrappers {
			if strings.HasPrefix(cmd, w+" ") {
				rest := strings.TrimPrefix(cmd, w+" ")
				// drop a leading numeric arg for timeout/nice (best-effort):
				// "30" or "-n 5" etc. — consume a single token after timeout.
				fields := strings.Fields(rest)
				if len(fields) >= 2 && (w == "timeout" || w == "nice" || w == "stdbuf") {
					// skip the first token only if it looks like a flag/number
					if fields[0][0] == '-' || isAllDigits(fields[0]) {
						rest = rest[len(fields[0]):]
						rest = strings.TrimSpace(rest)
					}
				}
				cmd = strings.TrimSpace(rest)
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}
	return cmd
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var bashSeparators = []string{"&&", "||", ";", "|&", "|", "&"}

// splitCompound splits a compound command into subcommands on shell
// separators. Newlines also split.
func splitCompound(cmd string) []string {
	clean := strings.ReplaceAll(cmd, "\n", ";")
	parts := []string{clean}
	for _, sep := range bashSeparators {
		var next []string
		for _, p := range parts {
			for _, s := range strings.Split(p, sep) {
				if strings.TrimSpace(s) != "" {
					next = append(next, s)
				}
			}
		}
		parts = next
	}
	return parts
}

// FilePathMatcher matches file-path rules for read/edit/write/glob/grep.
// Specifiers use gitignore-style patterns with path anchors:
//
//	"/src/**"        project-relative (resolved against workspace)
//	"~/.*rc"         home-relative
//	"//abs/path/**"  filesystem-absolute
//	"*.env" / "**/.env"  relative to cwd, matches at any depth
//
// info must contain "path" (the call's target path, absolute) and optionally
// "workspace" and "home" for anchor resolution.
type FilePathMatcher struct{}

func (FilePathMatcher) Match(spec string, info map[string]any) (bool, error) {
	target, _ := info["path"].(string)
	if target == "" || spec == "" {
		return false, nil
	}
	workspace, _ := info["workspace"].(string)
	home, _ := info["home"].(string)
	return matchPathPattern(spec, target, workspace, home), nil
}

// matchPathPattern evaluates a gitignore-style path rule against an absolute
// target path. This is a pragmatic implementation (not a full gitignore
// engine): it handles the common anchors and * / ** wildcards.
func matchPathPattern(spec, target, workspace, home string) bool {
	var root string
	pat := spec
	switch {
	case strings.HasPrefix(spec, "//"):
		root = "/"
		pat = spec[1:] // "//abs" → "/abs"
	case strings.HasPrefix(spec, "~/"):
		root = home
		pat = strings.TrimPrefix(spec, "~")
	case strings.HasPrefix(spec, "/"):
		root = workspace
		pat = spec
	default:
		// bare pattern → matches at any depth (gitignore semantics)
		return matchGitignoreAnyDepth(spec, target)
	}
	abs := filepath.Join(root, pat)
	return matchGlob(abs, target)
}

// matchGitignoreAnyDepth matches a bare pattern (no anchor) against any path
// segment, like gitignore matching "**/pattern".
func matchGitignoreAnyDepth(spec, target string) bool {
	base := filepath.Base(target)
	if matchGlob(spec, base) {
		return true
	}
	// also try matching each parent directory's segment
	dir := filepath.Dir(target)
	for dir != "/" && dir != "." {
		if matchGlob(spec, filepath.Base(dir)) {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

// matchGlob does a filepath.Match with ** support (** crosses directories).
func matchGlob(pattern, name string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, name)
		return ok
	}
	// Convert ** to a regex-ish check: split on ** and ensure segments match
	// in order. Pragmatic: use filepath.Match on each **-split segment.
	segs := strings.Split(pattern, "**")
	if len(segs) == 0 {
		return pattern == name
	}
	// leading segment must be prefix
	if segs[0] != "" && !strings.HasPrefix(name, segs[0]) {
		return false
	}
	// trailing segment must be suffix
	last := segs[len(segs)-1]
	if last != "" && !strings.HasSuffix(name, last) {
		return false
	}
	// middle segments: appear in order somewhere
	search := name
	if segs[0] != "" {
		search = strings.TrimPrefix(search, segs[0])
	}
	for i := 1; i < len(segs)-1; i++ {
		if segs[i] == "" {
			continue
		}
		idx := strings.Index(search, segs[i])
		if idx < 0 {
			return false
		}
		search = search[idx+len(segs[i]):]
	}
	return true
}

// DomainMatcher matches web_fetch rules of the form "domain:host" or
// "domain:*.example.com". Matching is case-insensitive.
type DomainMatcher struct{}

func (DomainMatcher) Match(spec string, info map[string]any) (bool, error) {
	host, _ := info["domain"].(string)
	if host == "" {
		return false, nil
	}
	if !strings.HasPrefix(spec, "domain:") {
		return false, nil
	}
	rule := strings.ToLower(strings.TrimPrefix(spec, "domain:"))
	host = strings.ToLower(host)
	rule = strings.TrimSuffix(rule, ".")
	host = strings.TrimSuffix(host, ".")
	if rule == "*" {
		return true, nil
	}
	if strings.HasPrefix(rule, "*.") {
		suffix := strings.TrimPrefix(rule, "*.")
		// "*.github.com" matches subdomains but NOT the bare apex.
		return strings.HasSuffix(host, "."+suffix), nil
	}
	return host == rule, nil
}

// MCPMatcher matches MCP tool rules: "mcp__server" (all tools from server) or
// "mcp__server__tool". info must contain "tool" (the full canonical name).
type MCPMatcher struct{}

func (MCPMatcher) Match(spec string, info map[string]any) (bool, error) {
	tool, _ := info["tool"].(string)
	if tool == "" || spec == "" {
		return false, nil
	}
	if spec == tool {
		return true, nil
	}
	// "mcp__server__*" matches all tools of server; "mcp__server" matches
	// any tool whose prefix is "mcp__server__".
	if strings.HasSuffix(spec, "__*") {
		prefix := strings.TrimSuffix(spec, "__*") + "__"
		return strings.HasPrefix(tool, prefix), nil
	}
	if strings.HasSuffix(spec, "*") {
		return strings.HasPrefix(tool, strings.TrimSuffix(spec, "*")), nil
	}
	return false, nil
}

// DefaultMatchers returns the standard matcher set keyed by tool name.
func DefaultMatchers() map[string]SpecifierMatcher {
	return map[string]SpecifierMatcher{
		"bash":      BashMatcher{},
		"write":     FilePathMatcher{},
		"edit":      FilePathMatcher{},
		"read":      FilePathMatcher{},
		"list":      FilePathMatcher{},
		"tree":      FilePathMatcher{},
		"glob":      FilePathMatcher{},
		"grep":      FilePathMatcher{},
		"web_fetch": DomainMatcher{},
	}
}
