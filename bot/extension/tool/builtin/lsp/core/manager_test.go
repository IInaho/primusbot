package lspcore

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultSpecsInvariants(t *testing.T) {
	seen := map[string]string{}
	for lang, s := range DefaultSpecs() {
		if s.Command == "" || s.LanguageID == "" || len(s.Extensions) == 0 {
			t.Errorf("lang %q: incomplete spec %+v", lang, s)
		}
		for _, ext := range s.Extensions {
			if prev, dup := seen[ext]; dup {
				t.Errorf("extension %q claimed by both %q and %q", ext, prev, lang)
			}
			seen[ext] = lang
		}
	}
	if seen[".go"] != "go" || seen[".rs"] != "rust" || seen[".cpp"] != "cpp" || seen[".cs"] != "csharp" {
		t.Errorf("unexpected routing: %v", seen)
	}
}

func TestExtensionRouting(t *testing.T) {
	m := NewManager(t.TempDir(), map[string]ServerSpec{
		"elixir": {Command: "no-such-elixir-ls-xyz", LanguageID: "elixir", Extensions: []string{".ex", ".exs"}, InstallHint: "mix archive.install"},
	})
	defer m.Close()

	if _, err := m.resolve("a.ex"); !errors.As(err, new(*notInstalledError)) {
		t.Fatalf("configured-but-missing language should yield notInstalledError, got %v", err)
	}
	_, err := m.resolve("a.go")
	if err == nil || !strings.Contains(err.Error(), "no language server") {
		t.Fatalf("unconfigured extension should report no server, got %v", err)
	}
}
