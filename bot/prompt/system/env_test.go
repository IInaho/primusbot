package system

import (
	"strings"
	"testing"
)

func TestFormatCwd(t *testing.T) {
	s := FormatCwd("/home/user")
	if s != "<cwd>/home/user</cwd>" {
		t.Errorf("FormatCwd = %q", s)
	}
}

func TestFormatEnv(t *testing.T) {
	s := FormatEnv("/home", "2026-01-01", "linux", "amd64")
	if !strings.Contains(s, "<env>") ||
		!strings.Contains(s, "<cwd>") ||
		!strings.Contains(s, "<date>") ||
		!strings.Contains(s, "<os>") ||
		!strings.Contains(s, "<arch>") {
		t.Errorf("FormatEnv = %q", s)
	}
}
