package lspcore

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLocations(t *testing.T) {
	single := `{"uri":"file:///a","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}}}`
	if got := parseLocations(json.RawMessage(single)); len(got) != 1 || got[0].URI != "file:///a" {
		t.Errorf("single: %+v", got)
	}
	arr := `[{"uri":"file:///a","range":{}},{"uri":"file:///b","range":{}}]`
	if got := parseLocations(json.RawMessage(arr)); len(got) != 2 {
		t.Errorf("array: %+v", got)
	}
	link := `[{"targetUri":"file:///c","targetRange":{"start":{"line":2,"character":0},"end":{"line":2,"character":1}}}]`
	got := parseLocations(json.RawMessage(link))
	if len(got) != 1 || got[0].URI != "file:///c" || got[0].Range.Start.Line != 2 {
		t.Errorf("locationlink: %+v", got)
	}
	if parseLocations(json.RawMessage("null")) != nil {
		t.Error("null should yield nil")
	}
}

func TestParseHover(t *testing.T) {
	markup := `{"contents":{"kind":"markdown","value":"func F()"}}`
	if got := parseHover(json.RawMessage(markup)); got != "func F()" {
		t.Errorf("markup hover = %q", got)
	}
	marked := `{"contents":[{"language":"go","value":"func F()"},"docs"]}`
	if got := parseHover(json.RawMessage(marked)); got != "func F()\ndocs" {
		t.Errorf("marked array hover = %q", got)
	}
	if got := parseHover(json.RawMessage(`{"contents":""}`)); got != "" {
		t.Errorf("empty hover = %q", got)
	}
}

func TestFormatDiagnostics(t *testing.T) {
	out := formatDiagnostics("main.go", "main.go", []Diagnostic{
		{Range: Range{Start: Position{Line: 3, Character: 2}}, Severity: 1, Code: "undefined", Message: "undefined: x"},
		{Range: Range{Start: Position{Line: 0, Character: 0}}, Severity: 2, Message: "unused"},
	})
	for _, want := range []string{
		"main.go: 2 problem(s) (1 error, 1 warning)",
		"4:3 error [undefined] undefined: x",
		"1:1 warning unused",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("formatDiagnostics missing %q:\n%s", want, out)
		}
	}
	if got := formatDiagnostics("main.go", "main.go", nil); got != "no problems found in main.go" {
		t.Errorf("empty diagnostics = %q", got)
	}
}

func TestFormatDiagnosticsImportHint(t *testing.T) {
	out := formatDiagnostics("client.py", "client.py", []Diagnostic{
		{Range: Range{Start: Position{Line: 0, Character: 0}}, Severity: 1, Code: "reportMissingImports", Message: `Import "httpx" could not be resolved`},
	})
	if !strings.Contains(out, "venv") || !strings.Contains(out, "reportMissingImports") {
		t.Errorf("import hint missing:\n%s", out)
	}
	// Unrelated errors must not trigger the environment-noise note.
	out = formatDiagnostics("main.go", "main.go", []Diagnostic{
		{Range: Range{Start: Position{Line: 0, Character: 0}}, Severity: 1, Message: "undefined: x"},
	})
	if strings.Contains(out, "venv") {
		t.Errorf("unexpected import hint:\n%s", out)
	}
}

func TestSameFileOnly(t *testing.T) {
	q := filepath.FromSlash("/w/main.go")
	if !sameFileOnly(nil, q) {
		t.Error("empty locations should be same-file-only")
	}
	if !sameFileOnly([]Location{{URI: pathToURI(q)}}, q) {
		t.Error("same file should be same-file-only")
	}
	if sameFileOnly([]Location{{URI: pathToURI(filepath.Join(filepath.Dir(q), "other.go"))}}, q) {
		t.Error("other file should not be same-file-only")
	}
}
