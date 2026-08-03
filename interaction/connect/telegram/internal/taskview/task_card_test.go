package taskview

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestTaskTrackerCollectsDiffWithoutPush(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_1")

	tracker.Track(controlruntime.Event{
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

	// tool_preview only feeds the run card — no push message is rendered
	// anymore (progress is not pushed; /diff shows the collected diff).
	diff := "[README.md#abc]\n--- a/README.md\n+++ b/README.md\n@@\n-old\n+new\n---\nEDIT_PREVIEW_JSON_B64 abc123"
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolPreview,
		Payload: controlruntime.ToolPayload{
			ToolName: "edit",
			Args:     `{"path":"README.md"}`,
			Preview:  diff,
		},
	})

	got := tracker.DiffSummary("")
	if !strings.Contains(got, "<b>差异</b>") || !strings.Contains(got, "+new") {
		t.Fatalf("diff summary not captured:\n%s", got)
	}
	if strings.Contains(got, "EDIT_PREVIEW_JSON_B64") || strings.Contains(got, "[README.md#abc]") {
		t.Fatalf("diff summary leaked internal payload:\n%s", got)
	}
}

func TestTaskTrackerEscapesTelegramHTML(t *testing.T) {
	tracker := NewTracker()
	tracker.Track(controlruntime.Event{
		RunID: "run_html",
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "edit <README> & report",
		},
	})
	tracker.Track(controlruntime.Event{
		RunID: "run_html",
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.RunResult{
			Output: "edited <README> & report",
		},
	})
	msg := tracker.DoneReply("run_html")
	if strings.Contains(msg, "<README>") || !strings.Contains(msg, "&lt;README&gt; &amp; report") {
		t.Fatalf("message did not escape HTML:\n%s", msg)
	}
}

func TestTaskTrackerDoneReplyShowsResultOnly(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_2")
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "run tests",
		},
	})
	tracker.Track(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunStarted})
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolStarted,
		Payload: controlruntime.ToolPayload{
			ToolName: "shell",
			Args:     `{"command":"go test ./..."}`,
		},
	})
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.RunResult{
			Output: "tests passed",
		},
	})
	done := tracker.DoneReply(runID)
	if !strings.Contains(done, "tests passed") {
		t.Fatalf("done reply missing result:\n%s", done)
	}
	if strings.Contains(done, "Done") || strings.Contains(done, "run tests") || strings.Contains(done, "步骤:") {
		t.Fatalf("done reply should not repeat status, user message, or counts:\n%s", done)
	}
}

func TestTaskTrackerSuppressesEmptyDoneReply(t *testing.T) {
	tracker := NewTracker()
	tracker.Track(controlruntime.Event{
		RunID: "run_empty_done",
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.RunResult{
			Output: "",
		},
	})
	if done := tracker.DoneReply("run_empty_done"); done != "" {
		t.Fatalf("empty done reply should be suppressed, got:\n%s", done)
	}
}

func TestStatusAndLastHaveDistinctScopes(t *testing.T) {
	tracker := NewTracker()
	runID := controlruntime.RunID("run_status")
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventInputAccepted,
		Payload: controlruntime.MessagePayload{
			Content: "update docs",
		},
	})
	tracker.Track(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunStarted})
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventToolStarted,
		Payload: controlruntime.ToolPayload{
			ToolName: "shell",
			Args:     `{"command":"go test ./..."}`,
		},
	})
	tracker.Track(controlruntime.Event{
		RunID: runID,
		Type:  controlruntime.EventRunDone,
		Payload: controlruntime.RunResult{
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

func TestApprovalMessage(t *testing.T) {
	msg := ApprovalMessage(controlruntime.ApprovalView{
		ID:       "apr_1",
		ToolName: "shell",
		Args: map[string]any{
			"command": "go test ./...",
		},
		Kind: "permission",
	})
	if !strings.Contains(msg, "<b>需要审批</b>") || strings.Contains(msg, "/approve") || !strings.Contains(msg, "<pre>go test ./...</pre>") {
		t.Fatalf("approval message missing fields:\n%s", msg)
	}
}

func TestQuestionMessageMultiPart(t *testing.T) {
	msg := QuestionMessage(controlruntime.QuestionView{
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
	})
	if !strings.Contains(msg, "/answer q_1") || !strings.Contains(msg, "1. Yes") {
		t.Fatalf("question message missing answer instructions:\n%s", msg)
	}
}

func TestQuestionMessageSelectableReliesOnButtons(t *testing.T) {
	msg := QuestionMessage(controlruntime.QuestionView{
		ID: "q_1",
		Questions: []controlruntime.QuestionItem{{
			Question: "Proceed?",
			Options:  []controlruntime.QuestionOption{{Label: "Yes"}, {Label: "No"}},
		}},
	})
	if strings.Contains(msg, "/answer") || strings.Contains(msg, "/dismiss") || strings.Contains(msg, "1. Yes") {
		t.Fatalf("single-choice question should rely on buttons:\n%s", msg)
	}
	if !strings.Contains(msg, "Proceed?") {
		t.Fatalf("question message missing body:\n%s", msg)
	}
}
