package shell

import (
	"os"
	"testing"

	"nekocode/bot/tools/runtime/sandbox"
	"nekocode/bot/tools/runtime/workspace"
)

// restoreWorkspace resets the package-global workspace state after the test.
// TempDir roots are deleted when the test ends; leaving them configured would
// make later sandbox mounts in this package fail on nonexistent paths.
func restoreWorkspace(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		cwd, _ := os.Getwd()
		workspace.Configure(cwd, nil)
	})
}

func TestApplyWorkspaceRoots(t *testing.T) {
	restoreWorkspace(t)
	cwd := t.TempDir()
	extraRW := t.TempDir()
	extraRO := t.TempDir()
	workspace.Configure(cwd, []workspace.Root{
		{Path: extraRW, Access: workspace.AccessReadWrite},
		{Path: extraRO, Access: workspace.AccessReadOnly},
	})

	profile := sandbox.Profile{Workspace: cwd}
	applyWorkspaceRoots(&profile, cwd)

	if len(profile.WritePaths) != 1 || profile.WritePaths[0] != extraRW {
		t.Fatalf("read-write root should become a WritePath, got %v", profile.WritePaths)
	}
	if len(profile.ReadPaths) != 1 || profile.ReadPaths[0] != extraRO {
		t.Fatalf("read-only root should become a ReadPath, got %v", profile.ReadPaths)
	}
}

func TestApplyWorkspaceRootsSkipsCwd(t *testing.T) {
	restoreWorkspace(t)
	cwd := t.TempDir()
	workspace.Configure(cwd, nil)

	profile := sandbox.Profile{Workspace: cwd}
	applyWorkspaceRoots(&profile, cwd)

	if len(profile.WritePaths) != 0 || len(profile.ReadPaths) != 0 {
		t.Fatalf("primary workspace must not be duplicated: write=%v read=%v",
			profile.WritePaths, profile.ReadPaths)
	}
}
