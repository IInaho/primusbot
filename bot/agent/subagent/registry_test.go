package subagent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBuiltinProfilesShareRuntimePromptAndSeparateCapabilities(t *testing.T) {
	coder, ok := GetProfile("coder")
	if !ok {
		t.Fatal("coder profile not found")
	}
	explore, ok := GetProfile("explore")
	if !ok {
		t.Fatal("explore profile not found")
	}
	if coder.SystemPrompt != explore.SystemPrompt {
		t.Fatal("built-in profiles should share the generic sub-agent prompt")
	}
	for _, want := range []string{"通用子 Agent", "实际可见工具是权限边界", "最终消息"} {
		if !strings.Contains(coder.SystemPrompt, want) {
			t.Errorf("generic prompt missing %q", want)
		}
	}
	if !slices.Contains(coder.Tools, "write") || !slices.Contains(coder.Tools, "shell") {
		t.Fatalf("coder tools = %v, want write and shell", coder.Tools)
	}
	for _, forbidden := range []string{"write", "edit", "shell", "process"} {
		if slices.Contains(explore.Tools, forbidden) {
			t.Errorf("explore unexpectedly allows %q", forbidden)
		}
	}
	for _, removed := range []string{"executor", "researcher", "verify"} {
		if _, ok := GetProfile(removed); ok {
			t.Errorf("legacy semantic role %q is still registered", removed)
		}
	}
}

func TestRegisterPluginAgent(t *testing.T) {
	def := AgentDef{
		Name:         "plugin-agent",
		SystemPrompt: "You are a plugin agent.",
		Tools:        []string{"Read", "Grep"},
	}

	at := def.ToProfile()
	RegisterPlugin(at)

	got, ok := GetProfile("plugin-agent")
	if !ok {
		t.Fatal("plugin agent not found in registry")
	}
	if got.Name != "plugin-agent" {
		t.Errorf("name = %q", got.Name)
	}

	UnregisterPlugin("plugin-agent")
	if _, ok := GetProfile("plugin-agent"); ok {
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

	at := def.ToProfile()
	if at.Name != "test-agent" {
		t.Errorf("Profile name = %q", at.Name)
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
