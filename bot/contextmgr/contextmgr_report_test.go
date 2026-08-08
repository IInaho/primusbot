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
	m.AddAssistant(types.Message{Content: "world"})
	m.BuildRequest(ModelRequest{})
	m.Add("user", "<workspace_event/>", types.MessageSourceRuntimeEvent)
	m.AddToolResultsBatch([]ToolResultMsg{{
		Message: types.Message{Content: "result", ToolCallID: "call_1"}, ToolName: "read",
	}})
	report := m.Report()
	if report.Budget != 10_000 || report.SystemPrompt == 0 {
		t.Fatalf("report budget/system = %+v", report)
	}
	if report.SystemPrompt != baseSystemTokens || report.SysInjections != 2 {
		t.Fatalf("runtime context should be a tagged user injection: base=%d report=%+v", baseSystemTokens, report)
	}
	if report.UserMessages != 1 || report.AssistantMsgs != 1 || report.ToolResults != 1 {
		t.Fatalf("report message counts = %+v", report)
	}
}
