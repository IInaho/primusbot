package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterPluginAgent(t *testing.T) {
	def := AgentDef{
		Name:         "plugin-agent",
		SystemPrompt: "You are a plugin agent.",
		Tools:        []string{"Read", "Grep"},
	}

	at := def.ToAgentType()
	RegisterPlugin(at)

	got, ok := Get("plugin-agent")
	if !ok {
		t.Fatal("plugin agent not found in registry")
	}
	if got.Name != "plugin-agent" {
		t.Errorf("name = %q", got.Name)
	}

	UnregisterPlugin("plugin-agent")
	if _, ok := Get("plugin-agent"); ok {
		t.Error("plugin agent should be gone after unregister")
	}
}

func TestParseAgentMD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-agent.md")
	content := `---
name: test-agent
description: A test agent
tools:
  - Read
  - Grep
  - Bash
---

# Test Agent

You are a test agent. Do your job.`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	def, err := ParseAgentMD(path)
	if err != nil {
		t.Fatalf("ParseAgentMD: %v", err)
	}
	if def.Name != "test-agent" {
		t.Errorf("name = %q, want test-agent", def.Name)
	}
	if len(def.Tools) != 3 {
		t.Errorf("tools len = %d, want 3", len(def.Tools))
	}
	if def.SystemPrompt != "# Test Agent\n\nYou are a test agent. Do your job." {
		t.Errorf("systemPrompt = %q", def.SystemPrompt)
	}

	at := def.ToAgentType()
	if at.Name != "test-agent" {
		t.Errorf("AgentType name = %q", at.Name)
	}
}

func TestParseAgentMD_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	os.WriteFile(path, []byte("just text"), 0o644)

	_, err := ParseAgentMD(path)
	if err == nil {
		t.Error("should fail without frontmatter")
	}
}

func TestParseAgentMD_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-name.md")
	os.WriteFile(path, []byte(`---
description: no name field
---
Body`), 0o644)

	_, err := ParseAgentMD(path)
	if err == nil {
		t.Error("should fail without name field")
	}
}
