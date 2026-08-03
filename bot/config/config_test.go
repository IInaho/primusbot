package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	if Default.Active != "default" {
		t.Errorf("expected Active 'default', got '%s'", Default.Active)
	}
	if len(Default.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(Default.Models))
	}
	m := Default.Models[0]
	if m.Provider != "deepseek" {
		t.Errorf("expected Provider 'deepseek', got '%s'", m.Provider)
	}
	if m.Model != "deepseek-v4-flash" {
		t.Errorf("expected Model 'deepseek-v4-flash', got '%s'", m.Model)
	}
	if m.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("expected BaseURL 'https://api.deepseek.com/v1', got '%s'", m.BaseURL)
	}
	if DefaultContextWindow != 128000 {
		t.Errorf("expected DefaultContextWindow 128000, got %d", DefaultContextWindow)
	}
	if Default.FlashModel != "" {
		t.Errorf("expected FlashModel empty, got %s", Default.FlashModel)
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Active != Default.Active {
		t.Errorf("expected Active '%s', got '%s'", Default.Active, cfg.Active)
	}
}

func TestLoad_ValidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".nekocode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	customCfg := Config{
		Active:     "claude",
		FlashModel: "deepseek-flash",
		Models: []ModelConfig{
			{Name: "default", Provider: "deepseek", APIKey: "sk-ds", Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"},
			{Name: "claude", Provider: "anthropic", APIKey: "sk-ant", Model: "claude-3-opus", BaseURL: "https://api.anthropic.com", Protocol: "anthropic"},
		},
	}

	data, err := json.Marshal(customCfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if info, err := os.Stat(configPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode after load = %v, %v; want 600", info, err)
	}

	if cfg.Active != customCfg.Active {
		t.Errorf("expected Active '%s', got '%s'", customCfg.Active, cfg.Active)
	}
	if cfg.FlashModel != customCfg.FlashModel {
		t.Errorf("expected FlashModel %s, got %s", customCfg.FlashModel, cfg.FlashModel)
	}
	if len(cfg.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(cfg.Models))
	}

	am := cfg.ActiveModelConfig()
	if am.Name != "claude" || am.Provider != "anthropic" || am.Model != "claude-3-opus" {
		t.Errorf("unexpected active config: %+v", am)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".nekocode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	if cfg.Active != Default.Active {
		t.Errorf("expected Active '%s', got '%s'", Default.Active, cfg.Active)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".nekocode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	partialJSON := `{"active": "gpt4", "models": [{"name": "gpt4", "provider": "openai", "api_key": "sk-xxx", "model": "gpt-4-turbo"}]}`
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(partialJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Active != "gpt4" {
		t.Errorf("expected Active 'gpt4', got '%s'", cfg.Active)
	}
	// The effective window resolves from the built-in table
	// (gpt-4-turbo → 128K).
	if cfg.EffectiveContextWindow() != 128000 {
		t.Errorf("expected effective ContextWindow 128000, got %d", cfg.EffectiveContextWindow())
	}
}

func TestConfig_JSONRoundTrip(t *testing.T) {
	original := Config{
		Active:     "default",
		FlashModel: "flash",
		Models: []ModelConfig{
			{Name: "default", Provider: "openai", APIKey: "sk-abc", Model: "gpt-4-turbo", BaseURL: "https://custom.api.com/v1"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if restored.Active != original.Active || restored.FlashModel != original.FlashModel {
		t.Errorf("round-trip failed: got %+v, want %+v", restored, original)
	}
	if len(restored.Models) != 1 || restored.Models[0].Name != "default" {
		t.Errorf("models round-trip failed")
	}
}

func TestConfigCloneDoesNotShareMutableState(t *testing.T) {
	original := Config{
		Models: []ModelConfig{{Name: "model"}},
		MCPServers: map[string]MCPServerConfig{
			"server": {Args: []string{"first"}, Env: map[string]string{"KEY": "value"}},
		},
		Permissions: &PermissionsConfig{
			Allow: []string{"read"},
			Sandbox: map[string]SandboxConfig{
				"shell": {WritableRoots: []string{"/work"}},
			},
		},
	}

	clone := original.Clone()
	clone.Models[0].Name = "changed"
	server := clone.MCPServers["server"]
	server.Args[0] = "changed"
	server.Env["KEY"] = "changed"
	clone.MCPServers["server"] = server
	clone.Permissions.Allow[0] = "changed"
	sandbox := clone.Permissions.Sandbox["shell"]
	sandbox.WritableRoots[0] = "/changed"
	clone.Permissions.Sandbox["shell"] = sandbox

	if original.Models[0].Name != "model" ||
		original.MCPServers["server"].Args[0] != "first" ||
		original.MCPServers["server"].Env["KEY"] != "value" ||
		original.Permissions.Allow[0] != "read" ||
		original.Permissions.Sandbox["shell"].WritableRoots[0] != "/work" {
		t.Fatalf("clone mutated original: %+v", original)
	}
}

func TestSave_CreatesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := Config{
		Active: "default",
		Models: []ModelConfig{
			{Name: "default", Provider: "openai", Model: "gpt-4o-mini", Protocol: "openai"},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if !Exists() {
		t.Fatal("expected config file to exist")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if loaded.Active != "default" {
		t.Fatalf("unexpected saved config: %+v", loaded)
	}
}

func TestValidate_DuplicateModelName(t *testing.T) {
	cfg := Config{
		Active: "default",
		Models: []ModelConfig{
			{Name: "default", Provider: "openai", Model: "gpt-4o-mini"},
			{Name: "default", Provider: "anthropic", Model: "claude-sonnet-4-5"},
		},
	}

	if err := Validate(&cfg); err == nil {
		t.Fatal("expected duplicate model name error")
	}
}

func TestConfig_ModelsList(t *testing.T) {
	cfg := Config{
		Active: "default",
		Models: []ModelConfig{
			{Name: "default", Provider: "deepseek", APIKey: "sk-ds", Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"},
			{Name: "claude", Provider: "anthropic", APIKey: "sk-ant", Model: "claude-sonnet-4-6", BaseURL: "https://api.anthropic.com", Protocol: "anthropic"},
			{Name: "gpt4", Provider: "openai", APIKey: "sk-openai", Model: "gpt-4-turbo"},
		},
	}

	names := cfg.AllModelNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 model names, got %d: %v", len(names), names)
	}
	if names[0] != "default" || names[1] != "claude" || names[2] != "gpt4" {
		t.Errorf("unexpected names: %v", names)
	}

	am := cfg.ActiveModelConfig()
	if am.Name != "default" || am.Provider != "deepseek" || am.Model != "deepseek-chat" {
		t.Errorf("unexpected active config: %+v", am)
	}

	if !cfg.SwitchModel("claude") {
		t.Fatal("SwitchModel(claude) failed")
	}
	if cfg.Active != "claude" {
		t.Errorf("expected Active 'claude', got '%s'", cfg.Active)
	}

	am = cfg.ActiveModelConfig()
	if am.Name != "claude" || am.Provider != "anthropic" || am.APIKey != "sk-ant" || am.Model != "claude-sonnet-4-6" {
		t.Errorf("SwitchModel(claude) active config wrong: %+v", am)
	}

	if !cfg.SwitchModel("gpt4") {
		t.Fatal("SwitchModel(gpt4) failed")
	}
	am = cfg.ActiveModelConfig()
	if am.Provider != "openai" || am.Model != "gpt-4-turbo" || am.APIKey != "sk-openai" {
		t.Errorf("SwitchModel(gpt4) active config wrong: %+v", am)
	}

	if cfg.SwitchModel("unknown") {
		t.Error("SwitchModel(unknown) should return false")
	}
}

func TestConfig_ModelsJSONRoundTrip(t *testing.T) {
	original := Config{
		Active: "claude",
		Models: []ModelConfig{
			{Name: "default", Provider: "deepseek", APIKey: "sk-ds", Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"},
			{Name: "claude", Provider: "anthropic", APIKey: "sk-ant", Model: "claude-sonnet-4-6", Protocol: "anthropic"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(restored.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(restored.Models))
	}
	if restored.Active != "claude" {
		t.Errorf("expected active 'claude', got %s", restored.Active)
	}
}
