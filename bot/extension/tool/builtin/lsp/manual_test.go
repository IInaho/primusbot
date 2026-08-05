//go:build manual

package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lspcore "nekocode/bot/extension/tool/builtin/lsp/core"
)

// TestLanguageServers drives each installed mainstream server against a
// per-language fixture under testdata/lsp. Servers not on PATH are skipped.
// Run with:
//
//	go test -tags manual -run TestLanguageServers -v ./bot/extension/tool/builtin/lsp/
func TestLanguageServers(t *testing.T) {
	specs := lspcore.DefaultSpecs()
	cases := []struct {
		lang, dir, file, callNeedle, symbol string
	}{
		{"go", "golang", "main.go", "_ = greet(", "greet"},
		{"rust", "rust", filepath.Join("src", "main.rs"), "let _ = greet(", "greet"},
		{"typescript", "typescript", "index.ts", "const msg = greet(", "greet"},
		{"python", "python", "main.py", "msg = greet(", "greet"},
		{"bash", "bash", "script.sh", `greet "world"`, "greet"},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			spec := specs[tc.lang]
			if _, err := exec.LookPath(spec.Command); err != nil {
				t.Skipf("%s not on PATH (%s)", spec.Command, spec.InstallHint)
			}
			root, err := filepath.Abs(filepath.Join("testdata", "lsp", tc.dir))
			if err != nil {
				t.Fatal(err)
			}
			m := lspcore.NewManager(root, specs)
			defer m.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
			defer cancel()

			line := findLine(t, root, tc.file, tc.callNeedle)
			def := defWithRetry(t, ctx, m, tc.file, line, tc.symbol, 150*time.Second)
			t.Logf("[%s] definition → %s", tc.lang, oneLine(def))
			if strings.Contains(def, "no definition") || !strings.Contains(def, filepath.Base(tc.file)) {
				t.Errorf("%s: definition did not resolve into %s: %s", tc.lang, tc.file, def)
			}

			if hov, err := m.Hover(ctx, tc.file, line, tc.symbol); err != nil {
				t.Errorf("%s hover: %v", tc.lang, err)
			} else {
				t.Logf("[%s] hover → %s", tc.lang, oneLine(hov))
			}
			if diag, err := m.Diagnostics(ctx, tc.file); err != nil {
				t.Errorf("%s diagnostics: %v", tc.lang, err)
			} else {
				t.Logf("[%s] diagnostics → %s", tc.lang, oneLine(diag))
			}
		})
	}
}

// defWithRetry polls Definition until it resolves or the budget elapses — some
// servers (rust-analyzer) keep indexing for a while after initialize returns.
func defWithRetry(t *testing.T, ctx context.Context, m *lspcore.Manager, file string, line int, symbol string, budget time.Duration) string {
	end := time.Now().Add(budget)
	for {
		d, err := m.Definition(ctx, file, line, symbol)
		if err != nil {
			// ContentModified means the server is still indexing — retry per LSP §-32801.
			if strings.Contains(err.Error(), "content modified") && time.Now().Before(end) {
				time.Sleep(time.Second)
				continue
			}
			t.Fatalf("definition: %v", err)
		}
		if !strings.Contains(d, "no definition") {
			return d
		}
		if time.Now().After(end) {
			return d
		}
		time.Sleep(time.Second)
	}
}

// TestReferencesCrossFile verifies that lsp_references finds call sites in
// other files, not just the definition's own file. This is the regression test
// for the "references only returns the definition" gap: a freshly started
// server answers the first query before its background workspace index has
// built, and referencesSettled must wait it out.
//
//	go test -tags manual -run TestReferencesCrossFile -v ./bot/extension/tool/builtin/lsp/
func TestReferencesCrossFile(t *testing.T) {
	specs := lspcore.DefaultSpecs()
	cases := []struct {
		lang, dir, defFile, callerFile, needle, symbol string
	}{
		{"go", "golang_multi", filepath.Join("greet", "greet.go"), filepath.Join("main", "main.go"), "func Greet(", "Greet"},
		{"rust", "rust_multi", filepath.Join("src", "lib.rs"), filepath.Join("src", "main.rs"), "pub fn greet(", "greet"},
		{"typescript", "ts_multi", "greet.ts", "main.ts", "export function greet(", "greet"},
		{"python", "python_pkg", "digest.py", "app.py", "def write_digest(", "write_digest"},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			spec := specs[tc.lang]
			if _, err := exec.LookPath(spec.Command); err != nil {
				t.Skipf("%s not on PATH (%s)", spec.Command, spec.InstallHint)
			}
			root, err := filepath.Abs(filepath.Join("testdata", "lsp", tc.dir))
			if err != nil {
				t.Fatal(err)
			}
			m := lspcore.NewManager(root, specs)
			defer m.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
			defer cancel()

			line := findLine(t, root, tc.defFile, tc.needle)
			refs, err := m.References(ctx, tc.defFile, line, tc.symbol)
			if err != nil {
				t.Fatalf("references: %v", err)
			}
			t.Logf("[%s] references → %s", tc.lang, oneLine(refs))
			if !strings.Contains(refs, filepath.ToSlash(tc.callerFile)) {
				t.Errorf("%s: references from %s did not include cross-file caller %s:\n%s", tc.lang, tc.defFile, tc.callerFile, refs)
			}
		})
	}
}

func findLine(t *testing.T, root, file, needle string) int {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("needle %q not found in %s", needle, file)
	return 0
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}
