package agent

import (
	"strings"
	"testing"

	"nekocode/bot/contextmgr"
	"nekocode/bot/policy"
	"nekocode/bot/provider/types"
	"nekocode/protocol"
)

func messagesContain(msgs []types.Message, substr string) bool {
	for _, msg := range msgs {
		if strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}

func TestBindInteractionRestoresPreviousCallbacks(t *testing.T) {
	originalCalls := 0
	activeCalls := 0
	a := newTestAgent()
	a.interaction = Interaction{
		Confirm: func(protocol.ConfirmRequest) protocol.ConfirmReply {
			originalCalls++
			return protocol.AllowOnce()
		},
	}

	release := a.BindInteraction(Interaction{
		Confirm: func(protocol.ConfirmRequest) protocol.ConfirmReply {
			activeCalls++
			return protocol.Deny()
		},
	})
	if reply := a.confirm(protocol.ConfirmRequest{}); reply.Allowed {
		t.Fatal("bound interaction did not replace the original callback")
	}
	release()
	if reply := a.confirm(protocol.ConfirmRequest{}); !reply.Allowed {
		t.Fatal("release did not restore the original callback")
	}
	if activeCalls != 1 || originalCalls != 1 {
		t.Fatalf("callback counts = active %d, original %d", activeCalls, originalCalls)
	}
}

func TestBindOutputRestoresPreviousCallbacks(t *testing.T) {
	a := newTestAgent()
	originalCalls := 0
	activeCalls := 0
	a.setOutput(Output{Text: func(string) { originalCalls++ }})

	release := a.BindOutput(Output{Text: func(string) { activeCalls++ }})
	a.stream.emitText("active")
	release()
	a.stream.emitText("original")
	if activeCalls != 1 || originalCalls != 1 {
		t.Fatalf("callback counts = active %d, original %d", activeCalls, originalCalls)
	}
}

func TestInjectHintAppendsTaggedUserContext(t *testing.T) {
	a := newTestAgent()
	before := a.deps.ctxMgr.Len()

	a.injectHint(&policy.Hint{Type: "final_check", Severity: "critical", Content: "run verification"})
	if got := a.deps.ctxMgr.Len(); got != before {
		t.Fatalf("hint changed history length: got %d, want %d", got, before)
	}

	a.applyTurnHints(nil)
	msgs := a.deps.ctxMgr.BuildRequest(contextmgr.ModelRequest{})
	if !messagesContain(msgs, `type="final_check"`) || !messagesContain(msgs, "run verification") {
		t.Fatalf("expected tagged final_check hint in build messages, got %+v", msgs)
	}
	if tail := msgs[len(msgs)-1]; tail.Role != "user" || tail.Source != types.MessageSourceHint {
		t.Fatalf("hint must be an internal user context message: %+v", tail)
	}

	a.deps.ctxMgr.SetHints("")
	a.deps.ctxMgr.BuildRequest(contextmgr.ModelRequest{})
	if got := a.deps.ctxMgr.Len(); got != before+1 {
		t.Fatalf("clearing hint appended context: history length = %d, want %d", got, before+1)
	}
}
