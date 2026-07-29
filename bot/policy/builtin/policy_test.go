package builtin

import (
	"strings"
	"testing"

	"nekocode/bot/policy"
)

type testState struct {
	ints map[string]int64
	strs map[string]string
	tool string
	args map[string]any
}

func newState() *testState {
	return &testState{ints: make(map[string]int64), strs: make(map[string]string)}
}

func (s *testState) Get(key string) int64                     { return s.ints[key] }
func (s *testState) Set(key string, value int64)              { s.ints[key] = value }
func (s *testState) Flag(key string) bool                     { return s.ints[key] == 1 }
func (s *testState) GetStr(key string) string                 { return s.strs[key] }
func (s *testState) ToolName() string                         { return s.tool }
func (s *testState) ToolArgs() map[string]any                 { return s.args }
func (s *testState) ToolError() bool                          { return false }
func (s *testState) SetStr(key, value string)                 { s.strs[key] = value }
func (s *testState) SetTool(name string, args map[string]any) { s.tool, s.args = name, args }

func TestQuotaHook(t *testing.T) {
	hk := QuotaHook()
	s := newState()

	s.Set(policy.StoreQuotaReads, 5)
	if r := hk.On(s); r != nil {
		t.Fatal("reads=5 should be silent")
	}
	s.Set(policy.StoreQuotaReads, 2)
	if r := hk.On(s); r == nil || r.Hint == nil || r.Hint.Severity != "warning" {
		t.Fatalf("reads=2 result = %+v, want warning hint", r)
	}
	if r := hk.On(s); r != nil {
		t.Fatal("same quota warning should dedupe")
	}
	s.Set(policy.StoreQuotaReads, 0)
	if r := hk.On(s); r == nil || r.Hint == nil || r.Hint.Severity != "critical" {
		t.Fatalf("reads=0 result = %+v, want critical hint", r)
	}
}

func TestVerificationHook(t *testing.T) {
	hk := VerificationHook()
	s := newState()

	s.Set(policy.StoreHasTasks, 0)
	if r := hk.On(s); r != nil {
		t.Fatal("no tasks should be silent")
	}
	s.Set(policy.StoreHasTasks, 1)
	s.Set(policy.StoreTasksAllDone, 1)
	if r := hk.On(s); r != nil {
		t.Fatal("all tasks done should be silent")
	}
	s.Set(policy.StoreTasksAllDone, 0)
	if r := hk.On(s); r == nil || r.BlockFinal == nil || !strings.Contains(r.BlockFinal.Reason, "未完成") {
		t.Fatalf("unfinished no-tool result = %+v, want block final", r)
	}
	if r := hk.On(s); r != nil {
		t.Fatal("verification warning should dedupe")
	}
}

func TestVerificationHookUsesStructuredFinalIntent(t *testing.T) {
	hk := VerificationHook()
	s := newState()
	s.Set(policy.StoreHasTasks, 1)
	s.Set(policy.StoreTasksAllDone, 0)
	s.Set(policy.StoreTurnToolCalls, 0)

	s.SetStr(policy.StoreFinalIntent, policy.FinalIntentFormatError)
	if r := hk.On(s); r != nil {
		t.Fatalf("format error intent should be handled by garbled circuit breaker, got %+v", r)
	}

	s.SetStr(policy.StoreFinalIntent, policy.FinalIntentNonFinal)
	if r := hk.On(s); r != nil {
		t.Fatalf("non-final intent should be silent, got %+v", r)
	}

	s.SetStr(policy.StoreFinalIntent, policy.FinalIntentFinal)
	if r := hk.On(s); r == nil || r.BlockFinal == nil {
		t.Fatalf("final intent with unfinished tasks should block, got %+v", r)
	}
}

func TestGarbledCircuitBreaker(t *testing.T) {
	hk := GarbledCircuitBreaker()
	s := newState()

	s.Set(policy.StoreRespGarbled, 4)
	if r := hk.On(s); r != nil {
		t.Fatal("count=4 should not stop")
	}
	s.Set(policy.StoreRespGarbled, 5)
	if r := hk.On(s); r == nil || r.Stop == nil || *r.Stop != policy.StopFormatError {
		t.Fatalf("count=5 result = %+v, want format stop", r)
	}
}

func TestProgressStallHook(t *testing.T) {
	hk := ProgressStallHook()
	s := newState()
	s.SetStr(policy.StoreStepInput, "test task")
	s.Set(policy.StoreTurnToolCalls, 1)

	for i := 0; i < 7; i++ {
		if r := hk.On(s); r != nil {
			t.Fatalf("stall turn %d result = %+v, want silent", i+1, r)
		}
	}
	if r := hk.On(s); r == nil || r.Hint == nil || r.RequireTool != nil {
		t.Fatalf("8th stall result = %+v, want warning hint", r)
	}
}

func TestExplorationHooks(t *testing.T) {
	exhausted := ExplorationExhaustedHook()
	s := newState()
	s.SetStr(policy.StoreStepInput, "test task")
	s.Set(policy.StoreExploreCalls, 10)

	r := exhausted.On(s)
	if r == nil || r.Hint == nil || r.RequireTool != nil {
		t.Fatalf("exploration exhausted result = %+v, want hint without required tool", r)
	}
	if r.StatePatch == nil || r.StatePatch.Ints[policy.PolicyExploreExhausted] != 1 {
		t.Fatalf("state patch = %+v, want explore exhausted policy", r.StatePatch)
	}
}

func TestExploreCascadeHook(t *testing.T) {
	hk := ExploreCascadeHook()
	s := newState()
	s.SetStr(policy.StoreStepInput, "test task")

	s.Set(policy.StoreToolResearcher, 3)
	if r := hk.On(s); r != nil {
		t.Fatal("3 researchers should be silent")
	}
	s.Set(policy.StoreToolResearcher, 4)
	if r := hk.On(s); r == nil || r.Hint == nil || r.Hint.Type != "explore_cascade" {
		t.Fatalf("4 researchers result = %+v, want cascade hint", r)
	}
}

func TestToolResultGuardrailHook(t *testing.T) {
	hk := ToolResultGuardrailHook()
	s := newState()

	s.Set(policy.StoreToolResultCount, 40)
	if r := hk.On(s); r != nil {
		t.Fatalf("threshold should be silent, got %+v", r)
	}
	s.Set(policy.StoreToolResultCount, 41)
	if r := hk.On(s); r == nil || r.Hint == nil || r.Hint.Type != "tool_results" {
		t.Fatalf("tool result guardrail = %+v, want hint", r)
	}
	if s.Get(policy.CounterToolResultWarned) != 41 {
		t.Fatalf("last warned = %d, want 41", s.Get(policy.CounterToolResultWarned))
	}
	s.Set(policy.StoreToolResultCount, 45)
	if r := hk.On(s); r != nil {
		t.Fatalf("interval should dedupe, got %+v", r)
	}
}

func TestReadBeforeWriteHook(t *testing.T) {
	hk := ReadBeforeWriteHook()
	s := newState()
	s.SetTool("edit", nil)
	s.SetStr(policy.StoreEditTargetPath, "main.go")
	s.Set(policy.StoreEditTargetExists, 1)
	s.Set(policy.StoreEditTargetWasRead, 0)
	s.Set(policy.StoreEditAnchorSufficient, 0)

	if r := hk.On(s); r == nil || r.BlockTool == nil || !strings.Contains(r.BlockTool.Reason, "main.go") {
		t.Fatalf("unread edit result = %+v, want block", r)
	}

	s.Set(policy.StoreEditTargetWasRead, 1)
	if r := hk.On(s); r != nil {
		t.Fatalf("read target should pass, got %+v", r)
	}

	s.Set(policy.StoreEditTargetWasRead, 0)
	s.Set(policy.StoreEditAnchorSufficient, 1)
	if r := hk.On(s); r != nil {
		t.Fatalf("anchored edit should pass, got %+v", r)
	}
}

func TestReadOnlySpiralHook(t *testing.T) {
	hk := ReadOnlySpiralHook()
	s := newState()

	s.Set(policy.StoreReadOnlyStreak, 2)
	if r := hk.On(s); r != nil {
		t.Fatalf("streak=2 should be silent, got %+v", r)
	}
	s.Set(policy.StoreReadOnlyStreak, 3)
	if r := hk.On(s); r == nil || r.Hint == nil || r.Hint.Type != "read_only_spiral" {
		t.Fatalf("streak=3 result = %+v, want hint", r)
	}
	if s.Get(policy.StoreReadOnlyStreak) != 0 {
		t.Fatalf("streak after hook = %d, want reset", s.Get(policy.StoreReadOnlyStreak))
	}
}
