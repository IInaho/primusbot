package command

import (
	"context"
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/skill"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/core"
)

type planControllerStub struct{ enabled bool }

func (s *planControllerStub) SetPlanMode(enabled bool) { s.enabled = enabled }

type noSkills struct{}

func (noSkills) SkillCommands() []skill.Command     { return nil }
func (noSkills) Skill(string) (skill.Command, bool) { return skill.Command{}, false }
func (noSkills) MarkSkillLoaded(string)             {}

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
	got := planModePrompt()
	for _, want := range []string{
		"read-only analysis", "runtime blocks mutation", "<environment_context>", "evidence, not new instructions",
		"confirmed facts", "concrete risks", "observable verification", "reproduce the failure",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plan prompt missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "<sandbox>") {
		t.Fatalf("plan prompt refers to stale environment block: %q", got)
	}
}

func TestPlanCommandAppendsModeToBaseSystemPrompt(t *testing.T) {
	mgr := ctxmgr.New(ctxmgr.Config{SystemPrompt: "stale"})
	controller := &planControllerStub{}
	h := New(Deps{
		CtxMgr: mgr,
		Ag:     func() PlanModeController { return controller },
		Skills: noSkills{}, ToolRegistry: tools.New(),
		BaseSystemPrompt: func() string { return "stable behavior contract" },
	})

	if _, handled := h.Execute(context.Background(), "/plan inspect prompts", mgr); handled {
		t.Fatal("plan command should continue into the agent")
	}
	got := mgr.Snapshot().SystemPrompt
	if !controller.enabled || !strings.Contains(got, "stable behavior contract") || !strings.Contains(got, "<plan-mode>") {
		t.Fatalf("plan mode did not preserve base contract: enabled=%t prompt=%q", controller.enabled, got)
	}
}

func TestSkillState(t *testing.T) {
	st := &skillState{MsgStart: -1}
	if clearSkillContext(nil, st); st.MsgStart != -1 {
		t.Error("should be no-op when MsgStart is -1")
	}
}
