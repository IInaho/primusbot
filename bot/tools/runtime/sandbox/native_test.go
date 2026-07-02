//go:build linux
// +build linux

package sandbox

import (
	"runtime"
	"strings"
	"testing"
)

func TestIsNativeAvailableRequiresLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	// We just verify the function doesn't panic; result depends on kernel config.
	_ = IsNativeAvailable()
}

func TestRunNativeBashRejectsEmptyWorkspace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	_, err := RunNativeBash(t.Context(), "echo ok", BashProfile{}, 0)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	if !strings.Contains(err.Error(), "workspace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNativeBashRejectsIllegalCachePath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	_, err := RunNativeBash(t.Context(), "echo ok", BashProfile{
		Workspace:  t.TempDir(),
		CachePaths: []string{"~/.ssh"},
	}, 0)
	if err == nil {
		t.Fatal("expected error for illegal cache path")
	}
}
