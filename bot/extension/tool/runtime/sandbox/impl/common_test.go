package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWritePaths_ValidPaths(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "npm cache",
			input: []string{filepath.Join(home, ".npm")},
			want:  []string{filepath.Join(home, ".npm")},
		},
		{
			name:  "arbitrary dir",
			input: []string{filepath.Join(home, "projects", "newapp")},
			want:  []string{filepath.Join(home, "projects", "newapp")},
		},
		{
			name:  "tilde npm",
			input: []string{"~/.npm"},
			want:  []string{filepath.Join(home, ".npm")},
		},
		{
			name:  "absolute path",
			input: []string{"/tmp/foo"},
			want:  []string{"/tmp/foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWritePaths(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d paths, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("path[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestResolveWritePaths_Dedup(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	paths := []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".npm"),
		"~/.npm",
	}
	got, err := resolveWritePaths(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 path after dedup, got %d: %v", len(got), got)
	}
}

func TestResolveWritePaths_SkipDot(t *testing.T) {
	got, err := resolveWritePaths([]string{"."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 paths, got %d: %v", len(got), got)
	}
}

func TestTruncateCapturedOutputKeepsHeadAndTailLines(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&b, "line %d %s\n", i, strings.Repeat("x", maxOutputBytes/60))
	}

	got := truncateCapturedOutput(b.String())
	for _, want := range []string{"line 0 ", "line 49 ", "line 70 ", "line 119 "} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing retained line %q", want)
		}
	}
	for _, notWant := range []string{"line 50 ", "line 69 "} {
		if strings.Contains(got, notWant) {
			t.Fatalf("middle line %q should be truncated", notWant)
		}
	}
	if !strings.Contains(got, "20 lines truncated") {
		t.Fatalf("missing truncation marker: %q", got)
	}
}
