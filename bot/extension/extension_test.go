package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/policy"
	"nekocode/bot/tools"
)

func TestManagerOwnsPluginSkillLifecycle(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", filepath.Join(root, "home"))

	pluginDir := filepath.Join(root, ".nekocode", "plugins", "demo")
	if err := os.MkdirAll(filepath.Join(pluginDir, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pluginDir, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(pluginDir, ".claude-plugin", "plugin.json"),
		[]byte(`{"name":"demo","skills":["skills/demo"]}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(pluginDir, "skills", "demo", "SKILL.md"),
		[]byte("---\nname: demo-skill\ndescription: demo\n---\nDemo body.\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	registry := tools.NewRegistry()
	manager := New(ctxmgr.New(ctxmgr.Config{}), registry, policy.New(), 32_000)
	manager.Load()
	defer manager.Close()

	if got := manager.Snapshot(); len(got.Plugins) != 1 {
		t.Fatalf("plugins = %d, want 1", len(got.Plugins))
	}
	if _, ok := manager.Skill("demo-skill"); !ok {
		t.Fatal("plugin skill was not exposed through the extension entry point")
	}
	if !registry.Has("skill") {
		t.Fatal("skill tool was not registered")
	}

	if err := manager.SetPluginEnabled("demo", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Skill("demo-skill"); ok {
		t.Fatal("disabled plugin skill is still exposed")
	}

	if err := manager.SetPluginEnabled("demo", true); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Skill("demo-skill"); !ok {
		t.Fatal("re-enabled plugin skill was not restored")
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		t.Fatal(err)
	}
	manager.Reload()
	if got := manager.Snapshot(); len(got.Plugins) != 0 {
		t.Fatalf("reload retained a removed plugin: %+v", got.Plugins)
	}
	if _, ok := manager.Skill("demo-skill"); ok {
		t.Fatal("reload retained a removed plugin skill")
	}
}

func TestAsyncInstallAlwaysSignalsDone(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", filepath.Join(root, "home"))

	manager := New(ctxmgr.New(ctxmgr.Config{}), tools.NewRegistry(), policy.New(), 32_000)
	defer manager.Close()

	done := make(chan struct{}, 1)
	notifications := make(chan string, 1)
	manager.installPlugin([]string{"./missing-plugin", "--yes"}, InstallCallbacks{
		Notify: func(message string) { notifications <- message },
		Done:   func() { done <- struct{}{} },
	})

	select {
	case message := <-notifications:
		if !strings.Contains(message, "Install failed") {
			t.Fatalf("notification = %q", message)
		}
	case <-time.After(time.Second):
		t.Fatal("async install did not notify")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("async install did not signal completion")
	}
}
