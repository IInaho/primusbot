package permission

import (
	"net/url"
	"strings"
)

// BuildCallInfo extracts the matcher-relevant fields from a tool call's args
// into the callInfo map the engine passes to SpecifierMatcher.Match.
//
//   - bash / shell: {"command": args["command"]}
//   - file tools (read/edit/write/glob/grep/list/tree): {"path", "workspace", "home"}
//   - web_fetch: {"domain": host extracted from args["url"]}
//
// Tools without a known matcher just get an empty map (the engine treats a
// scoped rule with no matcher as non-matching; a bare rule still applies).
func BuildCallInfo(toolName string, args map[string]any, workspace, home string) map[string]any {
	switch toolName {
	case "bash", "shell":
		if cmd, _ := args["command"].(string); cmd != "" {
			return CallInfoForBash(cmd)
		}
	case "read", "edit", "write", "glob", "grep", "list", "tree":
		path, _ := args["path"].(string)
		if path == "" {
			// glob/grep use "pattern"; treat it as a path-ish input
			path, _ = args["pattern"].(string)
		}
		return CallInfoForPath(path, workspace, home)
	case "web_fetch", "webfetch":
		if u, _ := args["url"].(string); u != "" {
			return CallInfoForDomain(hostFromURL(u))
		}
	}
	// Unknown tool or missing field: empty map → a scoped rule can't match
	// (no matcher), but a bare "Tool" rule still applies.
	return map[string]any{}
}

// hostFromURL extracts the hostname from a URL string; returns the input
// unchanged if it is not a parseable URL.
func hostFromURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		// bare host or "example.com/path" → take the first segment
		if i := strings.IndexByte(s, '/'); i > 0 {
			return s[:i]
		}
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return u.Hostname()
}
