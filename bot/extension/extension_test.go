package extension

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nekocode/bot/command"
	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/tool"
	"nekocode/bot/policy"
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
		[]byte(`{"name":"demo plugin","skills":["skills/demo"]}`),
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

	registry := tools.New()
	contextManager := ctxmgr.New(ctxmgr.Config{})
	manager := New(Config{
		Context: contextManager, Tools: registry,
		Policy: policy.New(), ContextWindow: 32_000,
	})
	manager.Load()
	defer manager.Close()
	commands := command.New(command.Deps{
		CtxMgr: contextManager, ToolRegistry: registry,
		GetConfigFn: func() config.ModelConfig { return config.ModelConfig{} },
	})
	manager.RegisterCommands(commands, nil)
	menu, ok := commands.Menu(context.Background(), "/plugin")
	if !ok || menu.Title != "Plugin action" || len(menu.Items) != 6 {
		t.Fatalf("plugin action menu = %+v, %v", menu, ok)
	}
	menu, ok = commands.Menu(context.Background(), "/plugin disable")
	if !ok || len(menu.Items) != 1 || menu.Items[0].Value != "/plugin disable demo plugin" {
		t.Fatalf("plugin choice menu = %+v, %v", menu, ok)
	}
	if result, handled := commands.Execute(context.Background(), menu.Items[0].Value, contextManager); !handled || !strings.Contains(result, `Disabled plugin "demo plugin"`) {
		t.Fatalf("spaced plugin command = %q, %v", result, handled)
	}
	if result, handled := commands.Execute(context.Background(), "/plugin enable demo plugin", contextManager); !handled || !strings.Contains(result, `Enabled plugin "demo plugin"`) {
		t.Fatalf("spaced plugin re-enable = %q, %v", result, handled)
	}

	if got := manager.Snapshot(); len(got.Plugins) != 1 {
		t.Fatalf("plugins = %d, want 1", len(got.Plugins))
	}
	if _, ok := manager.Skill("demo-skill"); !ok {
		t.Fatal("plugin skill was not exposed through the extension entry point")
	}
	if !slices.Contains(commands.Names(), "$demo-skill") {
		t.Fatal("plugin skill command was not registered")
	}
	if !registry.Has("skill") {
		t.Fatal("skill tool was not registered")
	}

	if err := manager.SetPluginEnabled("demo plugin", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Skill("demo-skill"); ok {
		t.Fatal("disabled plugin skill is still exposed")
	}
	if slices.Contains(commands.Names(), "$demo-skill") {
		t.Fatal("disabled plugin skill command remained registered")
	}

	if err := manager.SetPluginEnabled("demo plugin", true); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Skill("demo-skill"); !ok {
		t.Fatal("re-enabled plugin skill was not restored")
	}
	if !slices.Contains(commands.Names(), "$demo-skill") {
		t.Fatal("re-enabled plugin skill command was not restored")
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
	if slices.Contains(commands.Names(), "$demo-skill") {
		t.Fatal("reload retained a removed plugin skill command")
	}
}

func TestInstallReturnsFailure(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", filepath.Join(root, "home"))

	manager := New(Config{
		Context: ctxmgr.New(ctxmgr.Config{}), Tools: tools.New(),
		Policy: policy.New(), ContextWindow: 32_000,
	})
	defer manager.Close()

	result := manager.installPlugin(context.Background(), []string{"./missing-plugin", "--yes"}, nil)
	if !strings.Contains(result, "Install failed") {
		t.Fatalf("result = %q", result)
	}
}
