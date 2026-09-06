package core

import (
	"testing"

	"nekocode/bot/config"
)

// seedPersistConfig writes a two-model config so SwitchModel has a target.
// Both models use deepseek-v4-flash, whose built-in profile supports
// non-auto reasoning efforts (needed by the effort persistence test).
func seedPersistConfig(t *testing.T) {
	t.Helper()
	cfg := config.Config{
		Active: "default",
		Models: []config.ModelConfig{
			{Name: "default", Provider: "deepseek", APIKey: "sk-test", Model: "deepseek-v4-flash"},
			{Name: "alt", Provider: "deepseek", APIKey: "sk-test", Model: "deepseek-v4-flash"},
		},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func newPersistTestBot(t *testing.T) *Bot {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	seedPersistConfig(t)
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Error(err)
		}
	})
	return b
}

func TestSwitchModelPersistsConfig(t *testing.T) {
	b := newPersistTestBot(t)

	if err := b.SwitchModel("alt"); err != nil {
		t.Fatalf("SwitchModel: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if loaded.Active != "alt" {
		t.Fatalf("persisted active = %q, want %q", loaded.Active, "alt")
	}
}

func TestSetReasoningEffortPersistsConfig(t *testing.T) {
	b := newPersistTestBot(t)

	if err := b.SetReasoningEffort("high"); err != nil {
		t.Fatalf("SetReasoningEffort: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := loaded.ActiveModelConfig().ReasoningEffort; got != "high" {
		t.Fatalf("persisted reasoning effort = %q, want %q", got, "high")
	}
}
