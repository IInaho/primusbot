package connect

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestTranslateDropsProgressEvents(t *testing.T) {
	progress := []controlruntime.EventType{
		controlruntime.EventInputAccepted,
		controlruntime.EventSystemMessage,
		controlruntime.EventReasoningDelta,
		controlruntime.EventPhaseChanged,
		controlruntime.EventToolStarted,
		controlruntime.EventToolBlocked,
		controlruntime.EventToolPreview,
		controlruntime.EventToolCompleted,
		controlruntime.EventSubAgentStarted,
		controlruntime.EventSubAgentEnded,
		controlruntime.EventTodosUpdated,
		controlruntime.EventRunStarted,
		controlruntime.EventSessionChanged,
		controlruntime.EventConnectorStatus,
		controlruntime.EventMetricsUpdated,
	}
	for _, typ := range progress {
		if got := Translate(controlruntime.Event{Type: typ}); got != nil {
			t.Fatalf("Translate(%s) = %v, want nil (progress events must not reach IM)", typ, got)
		}
	}
}

func TestTranslatePreviewAndResult(t *testing.T) {
	intents := Translate(controlruntime.Event{
		Type:    controlruntime.EventAssistantDelta,
		RunID:   "r1",
		Payload: controlruntime.DeltaPayload{Delta: "hello"},
	})
	if len(intents) != 1 || intents[0].Kind != IntentPreview || intents[0].Text != "hello" || intents[0].RunID != "r1" {
		t.Fatalf("delta intents = %+v", intents)
	}

	intents = Translate(controlruntime.Event{
		Type:    controlruntime.EventRunDone,
		RunID:   "r1",
		Payload: controlruntime.RunResult{Output: "done!"},
	})
	if len(intents) != 1 || intents[0].Kind != IntentResult || intents[0].Text != "done!" {
		t.Fatalf("run_done intents = %+v", intents)
	}

	// A failed run must always produce a message — silence on failure is the
	// worst possible IM behavior.
	intents = Translate(controlruntime.Event{
		Type:    controlruntime.EventRunFailed,
		RunID:   "r1",
		Payload: controlruntime.RunResult{Error: "boom"},
	})
	if len(intents) != 1 || intents[0].Kind != IntentFailed || !strings.Contains(intents[0].Text, "boom") {
		t.Fatalf("run_failed intents = %+v", intents)
	}

	intents = Translate(controlruntime.Event{Type: controlruntime.EventRunCancelled, RunID: "r1"})
	if len(intents) != 1 || intents[0].Kind != IntentStopped {
		t.Fatalf("run_aborted intents = %+v", intents)
	}
}

func TestTranslateApprovalIntent(t *testing.T) {
	view := controlruntime.ApprovalView{
		ID:                    "a1",
		ToolName:              "bash",
		Args:                  map[string]any{"command": "rm -rf /tmp/x"},
		CanEscalatePermission: true,
	}
	intents := Translate(controlruntime.Event{Type: controlruntime.EventApprovalRequested, Payload: view})
	if len(intents) != 1 {
		t.Fatalf("approval intents = %+v", intents)
	}
	in := intents[0]
	if in.Kind != IntentApproval || in.ID != "a1" || in.Approval == nil {
		t.Fatalf("approval intent = %+v", in)
	}
	if !strings.Contains(in.Text, "rm -rf") || !strings.Contains(in.Text, "/approve a1") {
		t.Fatalf("approval text = %q", in.Text)
	}
	var ids []string
	for _, a := range in.Actions {
		ids = append(ids, a.ID)
	}
	want := []string{ActionOnce, ActionAlways, ActionReject, ActionEscalate}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("action ids = %v, want %v", ids, want)
	}
}

func TestTranslateResolvedIntents(t *testing.T) {
	intents := Translate(controlruntime.Event{
		Type:    controlruntime.EventApprovalResolved,
		Payload: controlruntime.ApprovalView{ID: "a1", Status: controlruntime.ApprovalApproved},
	})
	if len(intents) != 1 || intents[0].Verdict != "已批准" {
		t.Fatalf("approval_resolved intents = %+v", intents)
	}
	intents = Translate(controlruntime.Event{
		Type:    controlruntime.EventQuestionResolved,
		Payload: controlruntime.QuestionView{ID: "q_1"},
	})
	if len(intents) != 1 || intents[0].Kind != IntentQuestionResolved || intents[0].ID != "q_1" {
		t.Fatalf("question_resolved intents = %+v", intents)
	}
}

func TestDeliverableGatesPreviewOnEditMessages(t *testing.T) {
	preview := Intent{Kind: IntentPreview}
	if deliverable(preview, Capabilities{}) {
		t.Fatal("preview must be dropped without EditMessages (multi-message streaming is spam)")
	}
	if !deliverable(preview, Capabilities{EditMessages: true}) {
		t.Fatal("preview must be delivered with EditMessages")
	}
	// Resolved intents go to every channel: text-only channels use them for
	// bookkeeping (clearing pending-question trackers).
	resolved := Intent{Kind: IntentQuestionResolved}
	if !deliverable(resolved, Capabilities{}) {
		t.Fatal("resolved intents must reach every channel")
	}
}

func TestApprovalDecisionAndVerdict(t *testing.T) {
	d, err := ApprovalDecisionFor(ActionAlways)
	if err != nil || !d.Allowed || !d.Remember {
		t.Fatalf("always decision = %+v, %v", d, err)
	}
	if _, err := ApprovalDecisionFor("bogus"); err == nil {
		t.Fatal("unknown action must error")
	}
	if VerdictForAction(ActionEscalate) != "已批准并授权" {
		t.Fatalf("escalate verdict = %q", VerdictForAction(ActionEscalate))
	}
	if !IsResolvedErr(errString("runtime: approval a1 already resolved")) {
		t.Fatal("already-resolved error not detected")
	}
	if IsResolvedErr(errString("connection refused")) {
		t.Fatal("unrelated error misclassified as resolved")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
