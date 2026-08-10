package subagent

import (
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/prompt"
	"nekocode/bot/provider/types"
)

func TestBuildTaskPromptKeepsHandoffOutOfSystemRole(t *testing.T) {
	cfg := RunConfig{
		AgentType: AgentType{
			Name:         "executor",
			SystemPrompt: "base prompt",
		},
		Prompt:  "current task",
		Handoff: "prior findings",
	}

	system := buildSystemPrompt(cfg)
	if strings.Contains(system, "prior findings") || strings.Contains(system, "current task") {
		t.Fatalf("task evidence leaked into system prompt: %q", system)
	}
	got := buildTaskPrompt(cfg)
	for _, want := range []string{"unverified evidence", "prior findings", "[Current delegated task]", "current task"} {
		if !strings.Contains(got, want) {
			t.Fatalf("task prompt = %q, want %q", got, want)
		}
	}
	if strings.Contains(system, "<cwd>") {
		t.Fatalf("volatile cwd leaked into stable system prompt: %q", system)
	}
}

func TestContextManagerRefreshesSandboxAtBuildTime(t *testing.T) {
	root := "/repo"
	cfg := RunConfig{
		AgentType: AgentType{
			Name:         "executor",
			SystemPrompt: "base prompt",
		},
		Environment: func() prompt.Environment {
			return prompt.Environment{
				Cwd: root, Roots: []prompt.Root{{Path: root, Access: "read-write"}},
			}
		},
	}

	e := &Engine{}
	mgr := e.newContextManager(cfg)
	first := mgr.BuildRequest(ctxmgr.ModelRequest{})
	if !messagesContainText(first, "<environment_context>") || !messagesContainText(first, `<root access="read-write">/repo</root>`) {
		t.Fatalf("first context missing environment block: %+v", first)
	}
	root = "/approved"
	second := mgr.BuildRequest(ctxmgr.ModelRequest{})
	if !messagesContainText(second, `<root access="read-write">/repo</root>`) || !messagesContainText(second, `<root access="read-write">/approved</root>`) {
		t.Fatalf("environment block did not refresh: %+v", second)
	}
	for i := range first {
		if first[i].Role != second[i].Role || first[i].Content != second[i].Content {
			t.Fatalf("environment refresh rewrote request prefix at %d: first=%+v second=%+v", i, first[i], second[i])
		}
	}
	if strings.Contains(mgr.Snapshot().SystemPrompt, "<environment_context>") {
		t.Fatal("subagent environment leaked into snapshot")
	}
}

func TestBuildSystemPromptNilWorkspace(t *testing.T) {
	cfg := RunConfig{
		AgentType: AgentType{Name: "executor", SystemPrompt: "base prompt"},
	}
	got := buildSystemPrompt(cfg)
	if strings.Contains(got, "<environment_context>") {
		t.Fatalf("nil workspace should not inject environment block: %q", got)
	}
}

func messagesContainText(msgs []types.Message, text string) bool {
	for _, msg := range msgs {
		if strings.Contains(msg.Content, text) {
			return true
		}
	}
	return false
}

func TestBuildSystemPromptExpandsDeepResearcher(t *testing.T) {
	cfg := RunConfig{
		AgentType: AgentType{
			Name:         "researcher",
			SystemPrompt: "research prompt",
		},
		Thoroughness: thoroughDeep,
	}

	got := buildSystemPrompt(cfg)
	if !strings.Contains(got, "<research_scope>") || !strings.Contains(got, "broad search") {
		t.Fatalf("system prompt = %q, want deep researcher instruction", got)
	}
}
