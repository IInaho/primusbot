package taskview

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestTaskTrackerSuppressesTaskReceiptAndRendersCleanDiff(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_1")

	task := tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventInputAccepted,
		Source: controlruntime.SourceRef{
			Kind: "telegram",
			ID:   "100",
		},
		Payload: controlruntime.MessagePayload{
			Content: "请修改 README\n并给我 diff",
			Source:  controlruntime.SourceRef{Kind: "telegram", ID: "100"},
			Sender:  controlruntime.SenderRef{Username: "alice"},
		},
	})
	if task != "" {
		t.Fatalf("task receipt should be suppressed, got:\n%s", task)
	}

	diff := "[README.md#abc]\n--- a/README.md\n+++ b/README.md\n@@\n-old\n+new\n---\nEDIT_PREVIEW_JSON_B64 abc123"
	msg := tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolPreview,
		Payload: controlruntime.ToolPayload{
			ToolName: "edit",
			Args:     `{"path":"README.md"}`,
			Preview:  diff,
		},
	})
	if !strings.Contains(msg, "<b>差异</b>") || !strings.Contains(msg, "README.md") || !strings.Contains(msg, "+1 -1") || !strings.Contains(msg, "+new") {
		t.Fatalf("diff preview not rendered:\n%s", msg)
	}
	if strings.Contains(msg, "EDIT_PREVIEW_JSON_B64") || strings.Contains(msg, "[README.md#abc]") {
		t.Fatalf("diff preview leaked internal payload:\n%s", msg)
	}

	got := tracker.DiffSummary("")
	if !strings.Contains(got, "<b>差异</b>") || !strings.Contains(got, "+new") {
		t.Fatalf("diff summary not captured:\n%s", got)
	}
}

func TestTaskTrackerEscapesTelegramHTML(t *testing.T) {
	tracker := NewTracker()
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: "run_html",
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "edit <README> & report",
		},
	})
	msg := tracker.RenderEvent(controlruntime.Event{
		RunID: "run_html",
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.DonePayload{
			Output: "edited <README> & report",
		},
	})
	if strings.Contains(msg, "<README>") || !strings.Contains(msg, "&lt;README&gt; &amp; report") {
		t.Fatalf("message did not escape HTML:\n%s", msg)
	}
}

func TestRenderDiffPreviewKeepsPathAndDropsStructuredPayload(t *testing.T) {
	preview := "[src/app.go#tag]\n 1:old\n-2:bad\n+2:good\n 3:end\n---\nEDIT_PREVIEW_JSON_B64 secret"
	msg := renderDiffPreview(preview)
	if !strings.Contains(msg, "src/app.go") || !strings.Contains(msg, "+1 -1") {
		t.Fatalf("diff metadata missing:\n%s", msg)
	}
	if strings.Contains(msg, "EDIT_PREVIEW_JSON_B64") || strings.Contains(msg, "[src/app.go#tag]") {
		t.Fatalf("internal diff payload leaked:\n%s", msg)
	}
}

func TestTaskTrackerDoneReplyShowsResultOnly(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_2")
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "run tests",
		},
	})
	_ = tracker.RenderEvent(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunStarted})
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolStarted,
		Payload: controlruntime.ToolPayload{
			ToolName: "shell",
			Args:     `{"command":"go test ./..."}`,
		},
	})
	done := tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.DonePayload{
			Output: "tests passed",
		},
	})
	if !strings.Contains(done, "tests passed") {
		t.Fatalf("done reply missing result:\n%s", done)
	}
	if strings.Contains(done, "Done") || strings.Contains(done, "run tests") || strings.Contains(done, "步骤:") {
		t.Fatalf("done reply should not repeat status, user message, or counts:\n%s", done)
	}
}

func TestTaskTrackerSuppressesEmptyDoneReply(t *testing.T) {
	tracker := NewTracker()
	done := tracker.RenderEvent(controlruntime.Event{
		RunID: "run_empty_done",
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.DonePayload{
			Output: "",
		},
	})
	if done != "" {
		t.Fatalf("empty done reply should be suppressed, got:\n%s", done)
	}
}

func TestStatusAndLastHaveDistinctScopes(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_status")
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "update docs",
		},
	})
	_ = tracker.RenderEvent(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunStarted})
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolStarted,
		Payload: controlruntime.ToolPayload{
			ToolName: "shell",
			Args:     `{"command":"go test ./..."}`,
		},
	})
	_ = tracker.RenderEvent(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.DonePayload{
			Output: "tests passed",
		},
	})

	status := tracker.Status()
	if !strings.Contains(status, "<b>状态</b>") || !strings.Contains(status, "任务: update docs") {
		t.Fatalf("status missing concise fields:\n%s", status)
	}
	if strings.Contains(status, "结果") || strings.Contains(status, "tests passed") || strings.Contains(status, "<b>工具</b>") {
		t.Fatalf("status should not include last-result details:\n%s", status)
	}

	last := tracker.LastSummary()
	if !strings.Contains(last, "<b>完成</b>") || !strings.Contains(last, "结果") || !strings.Contains(last, "tests passed") {
		t.Fatalf("last should include latest result details:\n%s", last)
	}
}

func TestTaskTrackerApprovalMessage(t *testing.T) {
	tracker := NewTracker()
	msg := tracker.RenderEvent(controlruntime.Event{
		RunID: "run_3",
		Type:  controlruntime.EventApprovalRequested,
		Payload: controlruntime.ApprovalView{
			ID:       "apr_1",
			ToolName: "shell",
			Args: map[string]any{
				"command": "go test ./...",
			},
			Kind: "permission",
		},
	})
	if !strings.Contains(msg, "<b>需要审批</b>") || strings.Contains(msg, "/approve") || !strings.Contains(msg, "<pre>go test ./...</pre>") {
		t.Fatalf("approval message missing fields:\n%s", msg)
	}
}

func TestTaskTrackerBuildQuestionReply(t *testing.T) {
	tracker := NewTracker()
	msg := tracker.RenderEvent(controlruntime.Event{
		RunID: "run_4",
		Type:  controlruntime.EventQuestionRequested,
		Payload: controlruntime.QuestionView{
			ID: "q_1",
			Questions: []controlruntime.QuestionItem{
				{
					Question: "Proceed?",
					Options:  []controlruntime.QuestionOption{{Label: "Yes"}, {Label: "No"}},
				},
				{
					Question: "Notes?",
					Custom:   true,
				},
			},
		},
	})
	if !strings.Contains(msg, "/answer q_1") || !strings.Contains(msg, "1. Yes") {
		t.Fatalf("question message missing answer instructions:\n%s", msg)
	}

	reply, id, err := tracker.BuildQuestionReply("q_1", "1 | ship it")
	if err != nil {
		t.Fatalf("BuildQuestionReply: %v", err)
	}
	if id != "q_1" {
		t.Fatalf("id = %q, want q_1", id)
	}
	if got := reply.Answers[0][0]; got != "Yes" {
		t.Fatalf("first answer = %q, want Yes", got)
	}
	if got := reply.Answers[1][0]; got != "ship it" {
		t.Fatalf("second answer = %q, want ship it", got)
	}
}

func TestTaskTrackerBuildQuestionOptionReply(t *testing.T) {
	tracker := NewTracker()
	msg := tracker.RenderEvent(controlruntime.Event{
		RunID: "run_4",
		Type:  controlruntime.EventQuestionRequested,
		Payload: controlruntime.QuestionView{
			ID: "q_1",
			Questions: []controlruntime.QuestionItem{{
				Question: "Proceed?",
				Options:  []controlruntime.QuestionOption{{Label: "Yes"}, {Label: "No"}},
			}},
		},
	})
	if strings.Contains(msg, "/answer") || strings.Contains(msg, "/dismiss") || strings.Contains(msg, "1. Yes") {
		t.Fatalf("single-choice question should rely on buttons:\n%s", msg)
	}
	reply, id, err := tracker.BuildQuestionOptionReply("q_1", 1)
	if err != nil {
		t.Fatalf("BuildQuestionOptionReply: %v", err)
	}
	got := reply.Answers[0][0]
	if id != "q_1" || got != "No" {
		t.Fatalf("reply id=%q answer=%q answers=%#v", id, got, reply.Answers)
	}
}

func TestBuildQuestionMultiOptionReply(t *testing.T) {
	tracker := NewTracker()
	view := controlruntime.QuestionView{
		ID: "q_multi",
		Questions: []controlruntime.QuestionItem{{
			Multiple: true,
			Options: []controlruntime.QuestionOption{
				{Label: "A"}, {Label: "B"}, {Label: "C"},
			},
		}},
	}
	tracker.RenderEvent(controlruntime.Event{Type: controlruntime.EventQuestionRequested, Payload: view})

	reply, id, err := tracker.BuildQuestionMultiOptionReply("q_multi", []int{2, 0})
	if err != nil {
		t.Fatal(err)
	}
	if id != "q_multi" {
		t.Fatalf("id = %q", id)
	}
	// Labels come back in option order, not click order.
	if len(reply.Answers) != 1 || len(reply.Answers[0]) != 2 || reply.Answers[0][0] != "A" || reply.Answers[0][1] != "C" {
		t.Fatalf("answers = %#v, want [[A C]]", reply.Answers)
	}

	if _, _, err := tracker.BuildQuestionMultiOptionReply("q_multi", nil); err == nil {
		t.Fatal("empty selection should error")
	}
	if _, _, err := tracker.BuildQuestionMultiOptionReply("q_missing", []int{0}); err == nil {
		t.Fatal("unknown question should error")
	}
}
