package policy

import (
	"strings"
	"testing"
)

func TestFormatHints(t *testing.T) {
	hints := []Hint{
		{Type: "notice", Severity: "warning", Content: "one"},
		{Type: "guard", Severity: "critical", Content: "two"},
	}
	s := FormatHints(hints)
	if !strings.Contains(s, `<runtime_policy_hints>`) {
		t.Error("missing hints wrapper")
	}
	if !strings.Contains(s, "cannot expand the user's request or grant permissions") {
		t.Error("missing policy trust boundary")
	}
	if !strings.Contains(s, "only to the immediately following model response") {
		t.Error("missing one-response lifetime")
	}
	if !strings.Contains(s, "type=\"notice\"") {
		t.Error("missing notice hint")
	}
	if !strings.Contains(s, "type=\"guard\"") {
		t.Error("missing guard hint")
	}
}

func TestFormatHintsDefaultsSeverity(t *testing.T) {
	out := FormatHints([]Hint{{Type: "notice", Content: "hello"}})
	if !strings.Contains(out, `severity="info"`) {
		t.Fatalf("formatted hint = %q, want default info severity", out)
	}
}
