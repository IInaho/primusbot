package runtime

import (
	"context"
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/hooks"
	"nekocode/bot/hooks/builtin"
	aggov "nekocode/bot/policy"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools"
)

// TestFinishRunPersistsPolicyBlockedFinalText reproduces the session-reload
// bug: when a PostTurn hook blocks the final answer via applyFinalPolicyBlock,
// the text was shown to the user (returned as FinalOutput) but never appended
// to the context, so /sessions <id> lost it.
func TestFinishRunPersistsPolicyBlockedFinalText(t *testing.T) {
	ctxMgr := ctxmgr.NewSub("test", 128000, nil)
	reg := tools.NewRegistry()
	a := New(context.Background(), ctxMgr, nil, reg)
	a.deps.gov = aggov.NewManager(hooks.NewRegistry())
	builtin.Register(a.deps.gov.HookReg)

	finalText := "• P0-1 Bash 安全机制：已实现\n• P0-2 权限系统：已实现"

	// Simulate the policy-block path: finalText set, but NOT persisted
	// (mirrors applyFinalPolicyBlock when the response gate gives up retrying).
	a.run.stopReason = hooks.StopCompleted
	a.run.finalText = finalText
	a.run.finalPersisted = false

	msgCountBefore := a.deps.ctxMgr.Len()
	result := a.loopRunner.finishRun(nil)

	if result.FinalOutput != finalText {
		t.Fatalf("FinalOutput = %q, want %q", result.FinalOutput, finalText)
	}
	if a.deps.ctxMgr.Len() != msgCountBefore+1 {
		t.Fatalf("expected final answer persisted to context, msgs %d → %d", msgCountBefore, a.deps.ctxMgr.Len())
	}
	last := lastAssistantContent(a.deps.ctxMgr.Build())
	if last != finalText {
		t.Fatalf("persisted assistant content = %q, want %q", last, finalText)
	}
}

// TestFinishRunDoesNotDoublePersistRecordableText ensures that when the final
// text was already recorded by a normal chat turn (recordable), finishRun does
// not append a duplicate assistant message.
func TestFinishRunDoesNotDoublePersistRecordableText(t *testing.T) {
	ctxMgr := ctxmgr.NewSub("test", 128000, nil)
	reg := tools.NewRegistry()
	a := New(context.Background(), ctxMgr, nil, reg)

	finalText := "all done"
	a.run.stopReason = hooks.StopCompleted
	a.run.finalText = finalText
	a.run.finalPersisted = true // recordReasoningText already persisted it

	msgCountBefore := a.deps.ctxMgr.Len()
	result := a.loopRunner.finishRun(nil)

	if result.FinalOutput != finalText {
		t.Fatalf("FinalOutput = %q, want %q", result.FinalOutput, finalText)
	}
	if a.deps.ctxMgr.Len() != msgCountBefore {
		t.Fatalf("expected no duplicate persist, msgs %d → %d", msgCountBefore, a.deps.ctxMgr.Len())
	}
}

func TestFinishRunDoesNotPersistNonRecordableText(t *testing.T) {
	ctxMgr := ctxmgr.NewSub("test", 128000, nil)
	reg := tools.NewRegistry()
	a := New(context.Background(), ctxMgr, nil, reg)

	a.turnRunner.recordReasoningText(&ReasoningResult{
		Action:      ActionChat,
		ActionInput: "draft answer from hook-retried turn",
	}, true)
	a.turnRunner.recordReasoningText(&ReasoningResult{
		Action:      ActionChat,
		ActionInput: "LLM call failed: timeout",
		IsError:     true,
	}, false)
	a.run.stopReason = hooks.StopCompleted

	msgCountBefore := a.deps.ctxMgr.Len()
	result := a.loopRunner.finishRun(nil)

	if result.FinalOutput != "LLM call failed: timeout" {
		t.Fatalf("FinalOutput = %q, want latest non-recordable text", result.FinalOutput)
	}
	if a.deps.ctxMgr.Len() != msgCountBefore {
		t.Fatalf("expected latest fallback to stay out of context, msgs %d → %d", msgCountBefore, a.deps.ctxMgr.Len())
	}
	last := lastAssistantContent(a.deps.ctxMgr.Build())
	if last != "draft answer from hook-retried turn" {
		t.Fatalf("persisted assistant content = %q, want previous recordable text", last)
	}
}

func TestPostToolStopClearsStaleFinalText(t *testing.T) {
	ctxMgr := ctxmgr.NewSub("test", 128000, nil)
	reg := tools.NewRegistry()
	a := New(context.Background(), ctxMgr, nil, reg)

	a.run.lastText = "previous text"
	a.run.finalText = "stale final"
	a.run.finalPersisted = true
	stop := hooks.StopCompleted
	a.applyPostToolHookResult(hooks.Result{Stop: &stop})

	if a.run.lastText != "" {
		t.Fatalf("lastText = %q, want cleared", a.run.lastText)
	}
	if a.run.finalText != "" {
		t.Fatalf("finalText = %q, want cleared", a.run.finalText)
	}
	if a.run.finalPersisted {
		t.Fatal("finalPersisted = true, want false")
	}
}

func lastAssistantContent(msgs []types.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return strings.TrimSpace(msgs[i].Content)
		}
	}
	return ""
}
