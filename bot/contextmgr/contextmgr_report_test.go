package contextmgr

import (
	"testing"

	"nekocode/bot/provider/types"
)

func TestManagerReport(t *testing.T) {
	baseSystemTokens := New(Config{SystemPrompt: "system", ContextWindow: 10_000}).Report().SystemPrompt
	m := New(Config{
		SystemPrompt: "system", ContextWindow: 10_000,
		RuntimePrompt: func() string { return "runtime environment metadata" },
	})
	m.Add("user", "hello")
	m.AddAssistantResponse("world", "")
	m.AddToolResultsBatch([]ToolResultMsg{{
		Message: types.Message{Content: "result", ToolCallID: "call_1"}, ToolName: "read",
	}})
	report := m.Report()
	if report.Budget != 10_000 || report.SystemPrompt == 0 {
		t.Fatalf("report budget/system = %+v", report)
	}
	if report.SystemPrompt <= baseSystemTokens {
		t.Fatalf("runtime prompt was not counted in system tokens: base=%d report=%d", baseSystemTokens, report.SystemPrompt)
	}
	if report.UserMessages != 1 || report.AssistantMsgs != 1 || report.ToolResults != 1 {
		t.Fatalf("report message counts = %+v", report)
	}
}
