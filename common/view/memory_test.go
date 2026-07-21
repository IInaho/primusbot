package view

import "testing"

func TestNewMemoryViewParsesCanonicalSections(t *testing.T) {
	memory := NewMemoryView("", "/tmp/memory.md", `[Project Memory]
## Tech Stack
- Go

## Active Goals
- converge runtime

## User Preferences
- concise replies`)

	if memory.Scope != MemoryScopeProject {
		t.Fatalf("scope = %q, want %q", memory.Scope, MemoryScopeProject)
	}
	if memory.Empty {
		t.Fatal("memory should not be empty")
	}
	if len(memory.Sections) != 5 {
		t.Fatalf("sections = %d, want 5", len(memory.Sections))
	}
	if memory.Sections[0].Key != "tech_stack" || memory.Sections[0].Content != "- Go" {
		t.Fatalf("tech stack section = %#v", memory.Sections[0])
	}
	if memory.Sections[1].Content != "- converge runtime" {
		t.Fatalf("active goals section = %#v", memory.Sections[1])
	}
	if !memory.Sections[2].Empty {
		t.Fatalf("missing completed tasks section should be empty: %#v", memory.Sections[2])
	}
	if memory.Sections[4].Content != "- concise replies" {
		t.Fatalf("preferences section = %#v", memory.Sections[4])
	}
}
