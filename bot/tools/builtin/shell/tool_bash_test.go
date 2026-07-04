package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/tools/runtime/core"
)

func TestBashTool(t *testing.T) {
	b := &BashTool{}

	out, err := b.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		if _, ok := err.(core.PermissionError); !ok {
			t.Fatalf("bash: %v", err)
		}
		out, err = b.ExecuteWithPermission(context.Background(), map[string]any{"command": "echo hello"}, core.PermissionRequest{
			Capabilities: []string{core.CapProcessHost},
		})
		if err != nil {
			t.Fatalf("privileged bash: %v", err)
		}
	}
	if out != "hello\n" {
		t.Errorf("output = %q, want %q", out, "hello\n")
	}

	_, err = b.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestBashToolHostExecutionWritesOutsideWorkspace(t *testing.T) {
	b := &BashTool{}
	workspace := t.TempDir()
	outside := t.TempDir()
	t.Chdir(workspace)

	target := filepath.Join(outside, "host-write.txt")
	_, err := b.ExecuteWithPermission(context.Background(), map[string]any{
		"command": "printf host > " + shellQuote(target),
	}, core.PermissionRequest{
		Capabilities: []string{core.CapProcessHost},
	})
	if err != nil {
		t.Fatalf("privileged bash: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read host-written file: %v", err)
	}
	if string(got) != "host" {
		t.Fatalf("host-written file = %q, want host", got)
	}
}

func TestBashToolDeclaresNetworkCapability(t *testing.T) {
	b := &BashTool{}

	// Declaring net.outbound without prior authorization should produce a
	// RequiredPermissionError (not run in sandbox, not run on host).
	_, err := b.Execute(context.Background(), map[string]any{
		"command":      "curl https://example.com",
		"capabilities": []any{"net.outbound"},
	})
	if err == nil {
		t.Fatal("expected RequiredPermissionError for declared capability")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if !containsCapability(req.Capabilities, core.CapNetOutbound) {
		t.Fatalf("expected CapNetOutbound in request, got %v", req.Capabilities)
	}
	if req.Scope != "project" {
		t.Fatalf("expected project scope for capability grant, got %q", req.Scope)
	}
}

func TestBashToolProcessHostAlwaysOnceScope(t *testing.T) {
	b := &BashTool{}

	_, err := b.Execute(context.Background(), map[string]any{
		"command":      "echo hi",
		"capabilities": []any{"process.host"},
	})
	if err == nil {
		t.Fatal("expected RequiredPermissionError for process.host")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if req.Scope != "once" {
		t.Fatalf("process.host must be scope=once (never persisted), got %q", req.Scope)
	}
	if !containsCapability(req.Capabilities, core.CapProcessHost) {
		t.Fatalf("expected CapProcessHost in request, got %v", req.Capabilities)
	}
}

func containsCapability(caps []string, target string) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
