package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/sandbox"
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

func TestBashToolDescriptionClarifiesPersistence(t *testing.T) {
	desc := (&BashTool{}).Description()
	for _, want := range []string{
		"Shell process state is NOT preserved",
		"Workspace filesystem changes ARE persistent",
		"/tmp is isolated per call and not persistent",
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("description missing %q: %s", want, desc)
		}
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

func TestBashToolExplicitNetworkRequiresPermission(t *testing.T) {
	b := &BashTool{}
	_, err := b.Execute(context.Background(), map[string]any{
		"command": "curl https://example.com",
		"network": true,
	})
	if err == nil {
		t.Fatal("expected RequiredPermissionError for explicit network")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if !containsCapability(req.Capabilities, core.CapNetOutbound) {
		t.Fatalf("expected CapNetOutbound in request, got %v", req.Capabilities)
	}
}

func TestBashToolExplicitHostAlwaysOnceScope(t *testing.T) {
	b := &BashTool{}
	_, err := b.Execute(context.Background(), map[string]any{
		"command":      "echo hi",
		"sandbox_mode": "host",
		"network":      true,
	})
	if err == nil {
		t.Fatal("expected RequiredPermissionError for host sandbox mode")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if req.Scope != "once" {
		t.Fatalf("host sandbox mode must be scope=once, got %q", req.Scope)
	}
	if !containsCapability(req.Capabilities, core.CapProcessHost) {
		t.Fatalf("expected CapProcessHost in request, got %v", req.Capabilities)
	}
	if len(req.Capabilities) != 1 {
		t.Fatalf("host request should ignore other sandbox openings, got %v", req.Capabilities)
	}
}

func TestBuildProfileFromRequestReadOnly(t *testing.T) {
	profile, err := buildProfileFromRequest("/workspace", sandboxRequest{Mode: "read-only"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Mode != sandbox.ModeReadOnly {
		t.Fatalf("Mode = %q, want read-only", profile.Mode)
	}
	if profile.Network {
		t.Fatal("read-only profile must not enable network by default")
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

// TestDefaultBashTimeoutIs120s guards against accidental regression of the
// 120s default. The previous 10s was too short for legitimate one-shot
// commands (medium npm builds, curl of a slow mirror, `go test` on a large
// pkg) and inconsistent with the documented default ("default 120000"). If
// you bump this, also update the timeout_ms parameter description in
// tool_bash.go so docs and reality stay in sync.
func TestDefaultBashTimeoutIs120s(t *testing.T) {
	if defaultBashTimeout != 120*time.Second {
		t.Fatalf("defaultBashTimeout = %v, want 120s", defaultBashTimeout)
	}
}
