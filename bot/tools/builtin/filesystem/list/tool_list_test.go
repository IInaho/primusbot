package list

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/tools/builtin/filesystem/testutil"
)

func TestListTool(t *testing.T) {
	td := testutil.SetupTemp(t)
	if err := os.WriteFile(filepath.Join(td, ".env"), []byte("TOKEN=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &ListTool{}

	out, err := l.Execute(context.Background(), map[string]any{"path": td})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if out == "" {
		t.Error("expected directory listing")
	}
	if !strings.Contains(out, ".env") {
		t.Fatalf("expected hidden file in listing, got:\n%s", out)
	}

	_, err = l.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}
