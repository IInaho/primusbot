package viewmodel

import (
	"testing"

	"nekocode/bot/config"
	"nekocode/bot/provider/types"
	"nekocode/bot/session"
	controlruntime "nekocode/runtime"
)

func TestModel(t *testing.T) {
	got := Model(config.ModelConfig{Provider: "openai", Model: "gpt"})
	if got.Provider != "openai" || got.Model != "gpt" {
		t.Fatalf("model view = %+v", got)
	}
}

func TestSessionSnapshot(t *testing.T) {
	got := SessionSnapshot(&session.Snapshot{
		ID: "s1", CWD: "/work", CreatedAt: 1, UpdatedAt: 2,
		Messages: []types.Message{{Role: "user", Content: "hello"}},
	})
	if got.ID != "s1" || got.CWD != "/work" || got.MsgCount != 1 {
		t.Fatalf("session view = %+v", got)
	}
	if got := SessionSnapshot(nil); got != (controlruntime.SessionMeta{}) {
		t.Fatalf("nil session view = %+v", got)
	}
}

func TestMemory(t *testing.T) {
	got := Memory("", "/memory.md", `[Project Memory]
## Tech Stack
- Go

## Active Goals
- converge runtime

## User Preferences
- concise replies`)
	if got.Scope != controlruntime.MemoryScopeProject || got.Path != "/memory.md" || got.Empty {
		t.Fatalf("memory view = %+v", got)
	}
	if len(got.Sections) != 5 {
		t.Fatalf("memory sections = %d, want 5", len(got.Sections))
	}
	if got.Sections[0].Content != "- Go" || got.Sections[1].Content != "- converge runtime" {
		t.Fatalf("parsed memory sections = %+v", got.Sections)
	}
	if !got.Sections[2].Empty || got.Sections[4].Content != "- concise replies" {
		t.Fatalf("optional memory sections = %+v", got.Sections)
	}
}
