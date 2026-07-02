package sandbox

import (
	"runtime"
	"strings"
	"testing"
)

func TestRunSandboxed_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("only verify non-Linux behavior on non-Linux")
	}
	_, err := RunSandboxed(t.Context(), "echo ok", BashProfile{}, 0)
	if err == nil {
		t.Fatal("expected error on non-Linux")
	}
	varUnavailable, ok := err.(UnavailableError)
	if !ok {
		t.Fatalf("expected UnavailableError, got %T: %v", err, err)
	}
	if !strings.Contains(varUnavailable.Reason, "Linux") {
		t.Fatalf("unexpected reason: %s", varUnavailable.Reason)
	}
}

func TestRunSandboxed_DelegatesToNative(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only on linux")
	}
	// Empty workspace should return error without invoking native sandbox.
	_, err := RunSandboxed(t.Context(), "echo ok", BashProfile{}, 0)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	if !strings.Contains(err.Error(), "workspace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
