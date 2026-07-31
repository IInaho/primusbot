package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsEmptyFile(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "missing.md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f == nil || f.Build() != "" {
		t.Fatalf("missing file = %#v, want empty memory", f)
	}
}

func TestBuildIncludesPopulatedSections(t *testing.T) {
	f := &File{TechStack: "- Go", ActiveGoals: "- release"}
	got := f.Build()
	if !strings.Contains(got, "## Tech Stack\n- Go") ||
		!strings.Contains(got, "## Active Goals\n- release") {
		t.Fatalf("Build() = %q", got)
	}
}

func TestLoadParsesSections(t *testing.T) {
	content := `## Tech Stack
- Go
- React

## Active Goals
- release v1

## Completed Tasks
- setup ci

## Key Architecture Map
- pkg/auth: auth module

## User Preferences
- use two spaces
`
	path := filepath.Join(t.TempDir(), "memory.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(f.TechStack, "Go") ||
		!strings.Contains(f.CompletedTasks, "setup ci") ||
		!strings.Contains(f.Preferences, "two spaces") {
		t.Fatalf("parsed memory = %#v", f)
	}
}

func TestDefaultPathUsesNekoCodeHome(t *testing.T) {
	if got := DefaultPath(); !strings.Contains(got, ".nekocode") {
		t.Fatalf("DefaultPath() = %q", got)
	}
}
