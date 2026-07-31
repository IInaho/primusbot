package subagent

import (
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/tools/runtime/core"
)

func TestBuildSystemPromptAddsHandoff(t *testing.T) {
	cfg := RunConfig{
		AgentType: AgentType{
			Name:         "executor",
			SystemPrompt: "base prompt",
		},
		Cwd:     "/repo",
		Handoff: "prior findings",
	}

	got := buildSystemPrompt(cfg)
	for _, want := range []string{"base prompt", "<cwd>/repo</cwd>", "<handoff>", "prior findings"} {
		if !strings.Contains(got, want) {
			t.Fatalf("system prompt = %q, want %q", got, want)
		}
	}
}

func TestBuildSystemPromptExpandsDeepResearcher(t *testing.T) {
	cfg := RunConfig{
		AgentType: AgentType{
			Name:         "researcher",
			SystemPrompt: `Focus on the specific question. For "very thorough": search across multiple directories and naming conventions.`,
		},
		Thoroughness: thoroughDeep,
	}

	got := buildSystemPrompt(cfg)
	if !strings.Contains(got, "Search across ALL packages") {
		t.Fatalf("system prompt = %q, want deep researcher instruction", got)
	}
}

func TestApplyReadOnlySpiralGuardInjectsReminderAfterThreeExploratoryBatches(t *testing.T) {
	ctxMgr := ctxmgr.New(ctxmgr.Config{SystemPrompt: "system", ContextWindow: 128000})
	state := newRunState()
	calls := []core.ToolCallItem{{Name: "read", Args: map[string]any{"path": "a.go"}}}

	before := ctxMgr.Len()
	applyReadOnlySpiralGuard(ctxMgr, calls, state)
	applyReadOnlySpiralGuard(ctxMgr, calls, state)
	if ctxMgr.Len() != before {
		t.Fatalf("reminder injected too early: len=%d before=%d", ctxMgr.Len(), before)
	}

	applyReadOnlySpiralGuard(ctxMgr, calls, state)
	if ctxMgr.Len() != before+1 {
		t.Fatalf("len=%d, want reminder after third read-only batch", ctxMgr.Len())
	}
	if state.readOnlyStreak != 0 {
		t.Fatalf("readOnlyStreak = %d, want reset", state.readOnlyStreak)
	}
}

func TestApplyReadOnlySpiralGuardResetsOnMutation(t *testing.T) {
	ctxMgr := ctxmgr.New(ctxmgr.Config{SystemPrompt: "system", ContextWindow: 128000})
	state := &runState{readOnlyStreak: 2}
	applyReadOnlySpiralGuard(ctxMgr, []core.ToolCallItem{{Name: "write", Args: map[string]any{"path": "a.go"}}}, state)
	if state.readOnlyStreak != 0 {
		t.Fatalf("readOnlyStreak = %d, want reset", state.readOnlyStreak)
	}
}
