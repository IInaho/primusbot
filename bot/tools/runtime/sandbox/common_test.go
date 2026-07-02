package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowedCachePaths_ValidPaths(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "npm cache",
			input:   []string{filepath.Join(home, ".npm")},
			wantErr: false,
		},
		{
			name:    "cargo cache",
			input:   []string{filepath.Join(home, ".cargo")},
			wantErr: false,
		},
		{
			name:    "nested go mod",
			input:   []string{filepath.Join(home, "go", "pkg", "mod", "github.com", "foo")},
			wantErr: false,
		},
		{
			name:    "pkgrn-store",
			input:   []string{filepath.Join(home, ".pnpm-store")},
			wantErr: false,
		},
		{
			name:    "yarn cache",
			input:   []string{filepath.Join(home, ".cache", "yarn")},
			wantErr: false,
		},
		{
			name:    "ssh rejected",
			input:   []string{filepath.Join(home, ".ssh")},
			wantErr: true,
		},
		{
			name:    "/etc rejected",
			input:   []string{"/etc/passwd"},
			wantErr: true,
		},
		{
			name:    "outside home rejected",
			input:   []string{"/tmp/evil"},
			wantErr: true,
		},
		{
			name:    "tilde npm",
			input:   []string{"~/.npm"},
			wantErr: false,
		},
		{
			name:    "tilde ssh rejected",
			input:   []string{"~/.ssh"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := allowedCachePaths(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAllowedCachePaths_Dedup(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	paths := []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".npm"),
	}
	got, err := allowedCachePaths(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 path after dedup, got %d", len(got))
	}
}

func TestAllowedCachePaths_SkipDot(t *testing.T) {
	got, err := allowedCachePaths([]string{"."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 paths, got %d", len(got))
	}
}

func TestIsAllowedCachePath_MatchRoot(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	if !isAllowedCachePath(home, filepath.Join(home, ".npm")) {
		t.Fatal(".npm should be allowed")
	}
	if !isAllowedCachePath(home, filepath.Join(home, ".npm", "foo", "bar")) {
		t.Fatal("nested .npm path should be allowed")
	}
	if isAllowedCachePath(home, filepath.Join(home, ".npm Evil")) {
		t.Fatal(".npm Evil should NOT be allowed")
	}
	if isAllowedCachePath(home, "/etc/passwd") {
		t.Fatal("/etc/passwd should NOT be allowed")
	}
}
