package read

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/tools/builtin/filesystem/testutil"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/toolutil"
)

func TestReadTool(t *testing.T) {
	td := testutil.SetupTemp(t)
	r := &ReadTool{}
	p := filepath.Join(td, "a.go")

	_, err := r.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing path")
	}

	out, err := r.Execute(testutil.Context(td), map[string]any{
		"path": p, "startLine": float64(1), "endLine": float64(5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("empty output")
	}
}

func TestReadToolRecordsSnapshotInExecutionState(t *testing.T) {
	td := testutil.SetupTemp(t)
	r := &ReadTool{}
	p := filepath.Join(td, "a.go")
	state := execution.NewExecutionState()
	ctx := execution.WithExecutionState(testutil.Context(td), state)

	_, err := r.Execute(ctx, map[string]any{
		"path": p, "startLine": float64(1), "endLine": float64(5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tag := toolutil.ComputeFileHash("package main\n\nfunc main() {}\n")
	if snap := state.SnapshotStore.ByHash(p, tag); snap == nil {
		t.Fatalf("expected snapshot %s in execution state", tag)
	}
}

func TestReadToolDoesNotEmitEditViewMetadata(t *testing.T) {
	td := testutil.SetupTemp(t)
	r := &ReadTool{}
	p := filepath.Join(td, "a.go")

	out, err := r.Execute(testutil.Context(td), map[string]any{
		"path": p, "startLine": float64(1), "endLine": float64(3),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "\nVIEW rev=") || strings.Contains(out, " window=W") {
		t.Fatalf("did not expect VIEW metadata in read output, got:\n%s", out)
	}
	if !strings.Contains(out, "[") || !strings.Contains(out, "#") || !strings.Contains(out, "1:package main") {
		t.Fatalf("expected tag header and line output, got:\n%s", out)
	}
}
