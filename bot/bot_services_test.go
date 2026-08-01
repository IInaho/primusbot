package bot

import (
	"testing"

	"nekocode/bot/config"
)

func TestSandboxAuthorityIncludesWorkspaceChanges(t *testing.T) {
	current := config.Default.Clone()
	next := current.Clone()
	if sandboxAuthorityChanged(&current, next) {
		t.Fatal("identical configuration changed sandbox authority")
	}
	next.Workspaces = []config.WorkspaceConfig{{Path: "/tmp/data", Access: "read-write"}}
	if !sandboxAuthorityChanged(&current, next) {
		t.Fatal("workspace change did not change sandbox authority")
	}
}
