package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nekocode/bot/tools/runtime/core"
)

func TestStorePersistsProjectGrant(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	req := core.PermissionRequest{
		Reason:       "needs network",
		Capabilities: []string{core.CapNetPublic, core.CapCacheWrite},
		Details: map[string]any{
			"workspace":    "/repo",
			"commandClass": "package-install",
		},
	}

	if err := store.AllowProject("bash", req); err != nil {
		t.Fatalf("AllowProject: %v", err)
	}
	if _, ok := store.Match("bash", req); !ok {
		t.Fatal("expected persisted grant to match")
	}
}

func TestStoreDoesNotPersistHostOrUnknownShell(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	for _, caps := range [][]string{{core.CapProcessHost}, {core.CapShellUnknown}} {
		req := core.PermissionRequest{
			Capabilities: caps,
			Details: map[string]any{
				"workspace":    "/repo",
				"commandClass": "unknown",
			},
		}
		if err := store.AllowProject("bash", req); err != nil {
			t.Fatalf("AllowProject(%v): %v", caps, err)
		}
		if _, ok := store.Match("bash", req); ok {
			t.Fatalf("grant %v must not be persisted", caps)
		}
	}
}

func TestStoreDenyTakesPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.json")
	store := NewStore(path)
	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetPublic},
		Details: map[string]any{
			"workspace":    "/repo",
			"commandClass": "network",
		},
	}
	f := permissionFile{
		Version: 1,
		Projects: map[string]permissionProject{
			"/repo": {
				Grants: []Grant{
					{
						ID:           "deny",
						Effect:       "deny",
						Tool:         "bash",
						CommandClass: "network",
						Capabilities: []string{core.CapNetPublic},
						Workspace:    "/repo",
						Scope:        "project",
						CreatedAt:    time.Now(),
					},
					{
						ID:           "allow",
						Effect:       "allow",
						Tool:         "bash",
						CommandClass: "network",
						Capabilities: []string{core.CapNetPublic, core.CapCacheWrite},
						Workspace:    "/repo",
						Scope:        "project",
						CreatedAt:    time.Now(),
					},
				},
			},
		},
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, denied := store.Denied("bash", req); !denied {
		t.Fatal("expected deny to match")
	}
	if _, ok := store.Match("bash", req); ok {
		t.Fatal("allow must not match when deny also matches")
	}
}
