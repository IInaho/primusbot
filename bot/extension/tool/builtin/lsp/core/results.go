package lspcore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// parseLocations decodes the three shapes textDocument/definition and
// /references may return: Location, Location[], or LocationLink[].
func parseLocations(raw json.RawMessage) []Location {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	if s[0] == '[' {
		var arr []Location
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 && arr[0].URI != "" {
			return arr
		}
		var links []struct {
			TargetURI   string `json:"targetUri"`
			TargetRange Range  `json:"targetRange"`
		}
		if json.Unmarshal(raw, &links) == nil {
			out := make([]Location, 0, len(links))
			for _, l := range links {
				if l.TargetURI != "" {
					out = append(out, Location{URI: l.TargetURI, Range: l.TargetRange})
				}
			}
			return out
		}
		return nil
	}
	var one Location
	if json.Unmarshal(raw, &one) == nil && one.URI != "" {
		return []Location{one}
	}
	return nil
}

// parseHover decodes a Hover.contents, which may be a MarkupContent, a single
// MarkedString, or an array of them.
func parseHover(raw json.RawMessage) string {
	var h struct {
		Contents json.RawMessage `json:"contents"`
	}
	if json.Unmarshal(raw, &h) != nil || len(h.Contents) == 0 {
		return ""
	}
	return strings.TrimSpace(markedToText(h.Contents))
}

func markedToText(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	switch s[0] {
	case '"':
		var str string
		_ = json.Unmarshal(raw, &str)
		return str
	case '{':
		var mc struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &mc) == nil && mc.Value != "" {
			return mc.Value
		}
		var ms struct {
			Language string `json:"language"`
			Value    string `json:"value"`
		}
		_ = json.Unmarshal(raw, &ms)
		return ms.Value
	case '[':
		var parts []json.RawMessage
		_ = json.Unmarshal(raw, &parts)
		var out []string
		for _, p := range parts {
			if t := markedToText(p); t != "" {
				out = append(out, t)
			}
		}
		return strings.Join(out, "\n")
	}
	return ""
}

var severityName = map[int]string{1: "error", 2: "warning", 3: "info", 4: "hint"}

// formatDiagnostics renders a file's diagnostics for the model: a severity
// summary, one entry per problem with the offending source line underneath,
// and a note when the problems look like unresolved imports — usually
// environment noise (the language server's environment lacks the project's venv/node_modules)
// rather than real bugs. display is what the model sees; abs is the readable
// path used to fetch source snippets.
func formatDiagnostics(display, abs string, diags []Diagnostic) string {
	if len(diags) == 0 {
		return "no problems found in " + display
	}
	diags = append([]Diagnostic(nil), diags...) // copy: sort must not disturb the client's cache
	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Severity != diags[j].Severity {
			return diags[i].Severity < diags[j].Severity
		}
		if diags[i].Range.Start.Line != diags[j].Range.Start.Line {
			return diags[i].Range.Start.Line < diags[j].Range.Start.Line
		}
		return diags[i].Range.Start.Character < diags[j].Range.Start.Character
	})

	var counts [5]int
	for _, d := range diags {
		counts[d.Severity]++
	}
	var parts []string
	for _, sev := range []int{1, 2, 3, 4} {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, severityName[sev]))
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d problem(s)", display, len(diags))
	if len(parts) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(parts, ", "))
	}
	unresolved := false
	for _, d := range diags {
		sev := severityName[d.Severity]
		if sev == "" {
			sev = "error"
		}
		code := ""
		if d.Code != "" {
			code = " [" + d.Code + "]"
		}
		fmt.Fprintf(&b, "\n%d:%d %s%s %s",
			d.Range.Start.Line+1, d.Range.Start.Character+1, sev, code,
			strings.TrimSpace(d.Message))
		if snippet := readLine(abs, d.Range.Start.Line); snippet != "" {
			fmt.Fprintf(&b, "\n    %s", snippet)
		}
		unresolved = unresolved || looksLikeUnresolvedImport(d)
	}
	if unresolved {
		b.WriteString("\n(note: unresolved-import errors may be false positives — the server's environment lacks the project's venv/node_modules)")
	}
	return b.String()
}

// looksLikeUnresolvedImport matches the messages servers emit when a dependency
// is not visible from the server's environment (pyright's reportMissingImports,
// tsserver's "Cannot find module", rust-analyzer's unresolved import).
func looksLikeUnresolvedImport(d Diagnostic) bool {
	msg := strings.ToLower(d.Message)
	for _, kw := range []string{
		"could not be resolved", "cannot find module", "unresolved import",
		"no module named", "unable to import", "no such file or directory",
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	code := strings.ToLower(d.Code)
	return strings.Contains(code, "import") || strings.Contains(code, "module")
}

// sameFileOnly reports whether every location is inside queryFile — the shape a
// still-indexing server returns before it has searched the whole workspace.
func sameFileOnly(locs []Location, queryFile string) bool {
	for _, l := range locs {
		if filepath.Clean(uriToPath(l.URI)) != queryFile {
			return false
		}
	}
	return true
}
