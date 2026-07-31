package command

import (
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/tools/runtime/core"
)

func TestEstimateToolDefTokens(t *testing.T) {
	descs := []core.Descriptor{
		{Name: "read", Description: "read files", Parameters: []core.Parameter{
			{Name: "path", Type: "string", Description: "file path"},
		}},
	}
	n := EstimateToolDefTokens(descs)
	if n <= 0 {
		t.Errorf("expected positive token count, got %d", n)
	}
}

func TestContextReportFormatting(t *testing.T) {
	report := ctxmgr.ContextReport{
		Budget: 10_000, SystemPrompt: 500, ToolDefTokens: 1_000,
		SkillList: 200, Messages: 3_000, ToolDefCount: 15, UserMessages: 5,
	}
	got := formatContextReport(report)
	for _, want := range []string{"⛁", "5.3k", "10.0k", "System", "15 tools"} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q: %s", want, got)
		}
	}
	if got := buildBar(0, nil, 10); got != "" {
		t.Fatalf("zero-budget bar = %q", got)
	}
}

func TestPlanModePrompt(t *testing.T) {
	got := planModePrompt("inspect only")
	if !strings.Contains(got, "inspect only") {
		t.Fatalf("missing task: %q", got)
	}
	if !strings.Contains(got, "BLOCKED: write, edit, bash") {
		t.Fatalf("missing blocked tool rule: %q", got)
	}
}

func TestSkillState(t *testing.T) {
	st := &skillState{MsgStart: -1}
	if clearSkillContext(nil, st); st.MsgStart != -1 {
		t.Error("should be no-op when MsgStart is -1")
	}
}
