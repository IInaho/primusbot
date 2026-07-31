package edit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekocode/bot/tools/runtime/execution"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "nekocode-edit-test-home-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestEditV2ExactUniqueReplacement(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "one\ntwo\nthree\n")

	result, err := (&EditTool{}).Execute(context.Background(), map[string]any{
		"path":      p,
		"oldString": "two",
		"newString": "TWO",
	})
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, result)
	}
	if got, want := readFile(t, p), "one\nTWO\nthree\n"; got != want {
		t.Fatalf("unexpected content:\n%s", got)
	}
	if !strings.Contains(result, "-2:two") || !strings.Contains(result, "+2:TWO") {
		t.Fatalf("expected diff in result, got:\n%s", result)
	}
}

func TestEditV2RejectsAmbiguousOldString(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "x\ntarget\ny\ntarget\n")

	_, err := (&EditTool{}).Execute(context.Background(), map[string]any{
		"path":      p,
		"oldString": "target",
		"newString": "changed",
	})
	if err == nil || !strings.Contains(err.Error(), "matched 2 times") || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
}

func TestEditV2ReplaceAllExact(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "a foo\nb foo\n")

	if _, err := (&EditTool{}).Execute(context.Background(), map[string]any{
		"path":       p,
		"oldString":  "foo",
		"newString":  "bar",
		"replaceAll": true,
	}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if got, want := readFile(t, p), "a bar\nb bar\n"; got != want {
		t.Fatalf("unexpected content:\n%s", got)
	}
	preview := (&EditTool{}).Preview(map[string]any{
		"path":       p,
		"oldString":  "bar",
		"newString":  "baz",
		"replaceAll": true,
	})
	if !strings.Contains(preview, "(2 replacements)") {
		t.Fatalf("expected replacement count in preview, got:\n%s", preview)
	}
}

func TestEditV2LineTrimFallback(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "func main() {\n    call()\n}\n")

	result, err := (&EditTool{}).Execute(context.Background(), map[string]any{
		"path":      p,
		"oldString": "func main() {\ncall()\n}",
		"newString": "func main() {\n    other()\n}\n",
	})
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if got, want := readFile(t, p), "func main() {\n    other()\n}\n"; got != want {
		t.Fatalf("unexpected content:\n%s", got)
	}
	if !strings.Contains(result, "matched via line-trim") {
		t.Fatalf("expected fallback note, got:\n%s", result)
	}
}

func TestEditV2LineTrimFallbackPreservesTrailingLineBreak(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "before\n    call()\nafter\n")

	if _, err := (&EditTool{}).Execute(context.Background(), map[string]any{
		"path":      p,
		"oldString": "before\ncall()",
		"newString": "before\nother()",
	}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if got, want := readFile(t, p), "before\nother()\nafter\n"; got != want {
		t.Fatalf("unexpected content:\n%s", got)
	}
}

func TestEditV2PreviewIncludesStructuredPayload(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	writeFile(t, p, "one\ntwo\n")

	preview := (&EditTool{}).Preview(map[string]any{
		"path":      p,
		"oldString": "two",
		"newString": "TWO",
	})
	if !strings.Contains(preview, structuredDiffMarker) {
		t.Fatalf("expected structured diff marker in preview, got:\n%s", preview)
	}
	if !strings.Contains(preview, "-2:two") || !strings.Contains(preview, "+2:TWO") {
		t.Fatalf("expected text diff to remain present, got:\n%s", preview)
	}
}

func TestEditV2Revert(t *testing.T) {
	td := t.TempDir()
	t.Setenv("NEKOCODE_WORKSPACE", td)
	p := filepath.Join(td, "file.txt")
	original := "one\ntwo\n"
	writeFile(t, p, original)
	ctx := execution.WithExecutionState(context.Background(), execution.NewExecutionState())

	if _, err := (&EditTool{}).Execute(ctx, map[string]any{
		"path":      p,
		"oldString": "two",
		"newString": "TWO",
	}); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	result, err := (&EditTool{}).Execute(ctx, map[string]any{"path": p, "revert": true})
	if err != nil {
		t.Fatalf("revert failed: %v", err)
	}
	if got := readFile(t, p); got != original {
		t.Fatalf("unexpected reverted content:\n%s", got)
	}
	if !strings.Contains(result, "-2:TWO") || !strings.Contains(result, "+2:two") {
		t.Fatalf("expected revert diff, got %q", result)
	}
	if strings.Contains(result, "Reverted to pre-edit state") || strings.Contains(result, "latest snapshot") {
		t.Fatalf("expected diff-only revert output, got %q", result)
	}
}

func TestPruneUndoSnapshots(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"old.pre-edit", "middle.pre-edit", "new.pre-edit"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}

	pruneUndoSnapshots(dir, 2)

	if _, err := os.Stat(filepath.Join(dir, "old.pre-edit")); !os.IsNotExist(err) {
		t.Fatalf("oldest snapshot was not removed: %v", err)
	}
	for _, name := range []string{"middle.pre-edit", "new.pre-edit"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, info.Mode().Perm())
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
