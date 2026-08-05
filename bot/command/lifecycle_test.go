package command

import (
	"context"
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/tool"
	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/runner"
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
		SkillList: 200, Messages: 3_000, ToolDefCount: 15, UserMessages: 5, AssistantMsgs: 4, ToolResults: 3,
		CacheHitTokens: 880, CacheMissTokens: 120, CacheHitRatio: 0.88,
		PrefixTurn: ctxmgr.PrefixTurnStats{
			Requests: 3, HitTokens: 880, MissTokens: 120,
			PeakMiss:  ctxmgr.PrefixCallStats{Request: 2, HitTokens: 100, MissTokens: 100, Parts: []string{"system", "tools"}},
			LowestHit: ctxmgr.PrefixCallStats{Request: 3, HitTokens: 20, MissTokens: 80, Parts: []string{"tail/provider"}},
		},
	}
	got := formatContextReport(report)
	for _, want := range []string{
		"Context Window", "Used 4.7k / 10.0k (47%) · Free 5.3k (53%)", "Breakdown", "System",
		"Conversation", "Tools      15", "Messages   12 · 5 user · 4 assistant · 3 tool results", "Summary    none",
		"⛂ Session", "⛂ Last turn", "3 calls", "⛃ Peak miss", "100 miss",
		"⛃ Lowest hit", "20% hit",
		"system prompt changed", "tool definitions changed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "#2") || strings.Contains(got, "#3") {
		t.Fatalf("report leaked internal model-call sequence: %s", got)
	}
	if got := buildBar(0, nil, 10); got != "" {
		t.Fatalf("zero-budget bar = %q", got)
	}
}

func TestFormatCountUsesThousandsSeparators(t *testing.T) {
	if got := formatCount(1004); got != "1,004" {
		t.Fatalf("formatCount(1004) = %q", got)
	}
}

func TestBuildBarSeparatesCells(t *testing.T) {
	got := buildBar(100, []barSegment{{size: 50, kind: "sys"}, {size: 50, kind: "free"}}, 8)
	if got != "[ ⛁ ⛁ ⛁ ⛁ ⛶ ⛶ ⛶ ⛶ ]" {
		t.Fatalf("spaced bar = %q", got)
	}
}

func TestBuildBarKeepsFixedWidthWithTinySegments(t *testing.T) {
	got := buildBar(1_000, []barSegment{
		{size: 1, kind: "sys"},
		{size: 1, kind: "tools"},
		{size: 1, kind: "skills"},
		{size: 1, kind: "msgs"},
		{size: 996, kind: "free"},
	}, 24)
	cells := strings.Fields(strings.Trim(got, "[] "))
	if len(cells) != 24 {
		t.Fatalf("bar cells = %d, want 24: %q", len(cells), got)
	}
	for _, marker := range []string{barChars["sys"], barChars["skills"], barChars["free"]} {
		if !strings.Contains(got, marker) {
			t.Fatalf("bar lost non-empty segment %q: %q", marker, got)
		}
	}
}

func TestFormatPrefixMissPartsExplainsStableTail(t *testing.T) {
	got := formatPrefixMissParts([]string{"tail/provider"})
	if got != "stable prefix unchanged; new content or provider cache" {
		t.Fatalf("tail/provider label = %q", got)
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
	registry := tools.New(fakePlanTool{})
	executor := runner.NewExecutor(registry)
	h := New(Deps{
		CtxMgr:           mgr,
		SetPlanMode:      executor.SetPlanMode,
		ToolRegistry:     registry,
		BaseSystemPrompt: func() string { return "stable behavior contract" },
	})

	if _, handled := h.Execute(context.Background(), "/plan inspect prompts", mgr); handled {
		t.Fatal("plan command should continue into the agent")
	}
	got := mgr.Snapshot().SystemPrompt
	blocked := executor.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "write-test"}})[0]
	if blocked.Error == "" || !strings.Contains(got, "stable behavior contract") || !strings.Contains(got, "<plan-mode>") {
		t.Fatalf("plan mode did not preserve base contract or block writes: result=%+v prompt=%q", blocked, got)
	}
}

type fakePlanTool struct{}

func (fakePlanTool) Name() string                                    { return "write-test" }
func (fakePlanTool) Description() string                             { return "test" }
func (fakePlanTool) Parameters() []core.Parameter                    { return nil }
func (fakePlanTool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }
func (fakePlanTool) Execute(context.Context, map[string]any) (string, error) {
	return "unexpected", nil
}

func TestSkillState(t *testing.T) {
	st := &skillState{MsgStart: -1}
	if clearSkillContext(nil, st); st.MsgStart != -1 {
		t.Error("should be no-op when MsgStart is -1")
	}
}
