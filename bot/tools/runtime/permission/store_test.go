package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nekocode/bot/tools/runtime/core"
)

func TestStorePersistsProjectGrant(t *testing.T) {
	store := NewStore(t.TempDir())
	req := core.PermissionRequest{
		Reason:       "needs network",
		Capabilities: []string{core.CapNetOutbound, core.CapFsWriteCache},
	}

	if err := store.Allow("bash", req); err != nil {
		t.Fatalf("AllowProject: %v", err)
	}
	if _, ok := store.Match("bash", req); !ok {
		t.Fatal("expected persisted grant to match")
	}
}

func TestStoreDoesNotPersistHostExecution(t *testing.T) {
	store := NewStore(t.TempDir())
	req := core.PermissionRequest{
		Capabilities: []string{core.CapProcessHost},
	}
	if err := store.Allow("bash", req); err != nil {
		t.Fatalf("AllowProject: %v", err)
	}
	if _, ok := store.Match("bash", req); ok {
		t.Fatal("process.host grant must not be persisted")
	}
}

func TestStoreDenyTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
	}
	f := permissionFile{
		Version: 1,
		Projects: map[string]permissionProject{
			dir: {
				Grants: []Grant{
					{
						Effect:       "deny",
						Tool:         "bash",
						Capabilities: []string{core.CapNetOutbound},
						Workspace:    dir,
					},
					{
						Effect:       "allow",
						Tool:         "bash",
						Capabilities: []string{core.CapNetOutbound, core.CapFsWriteCache},
						Workspace:    dir,
					},
				},
			},
		},
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.projectPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.projectPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, denied := store.Denied("bash", req); !denied {
		t.Fatal("expected deny to match")
	}
	if _, ok := store.Match("bash", req); ok {
		t.Fatal("allow must not match when deny also matches")
	}
}

func TestRememberRuleRejectsEmptySpecifier(t *testing.T) {
	store := NewStore(t.TempDir())
	err := store.RememberRule("/repo", Rule{Tool: "bash", Effect: EffectAllow})
	if err == nil {
		t.Fatal("empty remembered specifier should be rejected")
	}
	if got := store.RememberedRules("/repo"); len(got) != 0 {
		t.Fatalf("empty remembered rule should not persist, got %+v", got)
	}
}

func TestRememberRuleCanonicalizesDuplicates(t *testing.T) {
	store := NewStore(t.TempDir())
	rules := []Rule{
		{Tool: "Bash", Specifier: "  echo hi  ", Effect: EffectAllow},
		{Tool: "bash", Specifier: "echo hi", Effect: EffectAllow},
	}
	for _, r := range rules {
		if err := store.RememberRule("/repo", r); err != nil {
			t.Fatalf("RememberRule: %v", err)
		}
	}
	got := store.RememberedRules("/repo")
	if len(got) != 1 {
		t.Fatalf("expected one canonical remembered rule, got %+v", got)
	}
	if got[0].Tool != "bash" || got[0].Specifier != "echo hi" {
		t.Fatalf("unexpected canonical rule: %+v", got[0])
	}
}

func TestRememberRuleRejectsAutoBroadenedBashSpecifier(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.RememberRule("/repo", Rule{Tool: "bash", Specifier: "echo *", Effect: EffectAllow}); err != nil {
		t.Fatalf("command-scoped wildcard should be allowed: %v", err)
	}
	bad := []string{`echo "喵~ *`, "rm -rf *", "npm run:*"}
	for _, spec := range bad {
		if err := store.RememberRule("/repo", Rule{Tool: "bash", Specifier: spec, Effect: EffectAllow}); err == nil {
			t.Fatalf("expected remembered bash spec %q to be rejected", spec)
		}
	}
}
