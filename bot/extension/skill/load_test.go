package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverAndLoad(t *testing.T) {
	td := t.TempDir()
	sd := filepath.Join(td, "test-skill")
	os.MkdirAll(sd, 0755)
	os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill
when_to_use: legacy trigger field should be ignored
allowed-tools:
  - bash
  - read
context: fork
agent: executor
max_steps: 4
context_window: 8000
---

# Test Skill

This is the test skill body.
`), 0644)
	os.WriteFile(filepath.Join(sd, "helper.txt"), []byte("helper"), 0644)

	paths := discoverSkills([]string{td})
	if len(paths) != 1 {
		t.Fatalf("expected 1 discovered skill, got %d", len(paths))
	}

	sk, err := loadSkill(paths[0])
	if err != nil {
		t.Fatalf("loadSkill: %v", err)
	}

	if sk.Name != "test-skill" {
		t.Errorf("name = %q", sk.Name)
	}
	if sk.Description != "A test skill" {
		t.Errorf("description = %q", sk.Description)
	}
	if sk.Context != "fork" || sk.AgentType != "executor" {
		t.Errorf("context/agent mismatch")
	}
	if len(sk.AllowedTools) != 2 || sk.MaxSteps != 4 || sk.ContextWindow != 8000 {
		t.Errorf("execution fields wrong")
	}
	if sk.Content != "# Test Skill\n\nThis is the test skill body." {
		t.Errorf("content = %q", sk.Content)
	}
	if len(sk.Files) != 1 || !strings.HasSuffix(sk.Files[0], "helper.txt") {
		t.Errorf("files = %v", sk.Files)
	}
}

func TestLoadErrors(t *testing.T) {
	_, err := parseSkillContent("no frontmatter")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
	_, err = parseSkillContent("---\nname: x\n---")
	if err == nil {
		t.Error("expected error for missing description")
	}
	_, err = parseSkillContent("---\nname: x\n---\nbody")
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Error("expected missing description error")
	}
}

func TestDefaultDirs(t *testing.T) {
	dirs := defaultDirs()
	if len(dirs) != 2 {
		t.Errorf("expected 2 default dirs, got %d", len(dirs))
	}
}

func TestBundledSkills(t *testing.T) {
	got := bundledSkills()
	want := map[string][]string{
		"skill-creator": {"Create", "skill"},
		"hunt":          {"Diagnose", "root cause", "debug"},
		"think":         {"architecture", "plan", "before coding"},
		"check":         {"Review", "diff", "after implementation"},
	}
	if len(got) != len(want) {
		t.Fatalf("bundled skills = %d, want %d", len(got), len(want))
	}
	for _, sk := range got {
		triggers, ok := want[sk.Name]
		if !ok {
			t.Errorf("unexpected bundled skill %q", sk.Name)
			continue
		}
		if sk.Description == "" || sk.Content == "" {
			t.Errorf("bundled skill %q is incomplete", sk.Name)
		}
		for _, trigger := range triggers {
			if !strings.Contains(sk.Description, trigger) {
				t.Errorf("bundled skill %q description missing trigger %q", sk.Name, trigger)
			}
		}
		delete(want, sk.Name)
	}
	if len(want) != 0 {
		t.Errorf("missing bundled skills: %v", want)
	}
}
