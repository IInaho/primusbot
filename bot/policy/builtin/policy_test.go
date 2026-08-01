package builtin

import (
	"strings"
	"testing"

	"nekocode/bot/policy"
)

type testState struct {
	facts policy.Facts
	ints  map[string]int64
	strs  map[string]string
}

func newState() *testState {
	return &testState{
		ints: make(map[string]int64),
		strs: make(map[string]string),
	}
}

func (s *testState) Facts() policy.Facts             { return s.facts }
func (s *testState) Int(name string) int64           { return s.ints[name] }
func (s *testState) SetInt(name string, value int64) { s.ints[name] = value }
func (s *testState) String(name string) string       { return s.strs[name] }
func (s *testState) SetString(name, value string)    { s.strs[name] = value }

func TestQuotaHook(t *testing.T) {
	hook := QuotaHook()
	state := newState()

	state.facts.Turn.ReadsLeft = 5
	if result := hook.On(state); result != nil {
		t.Fatal("reads=5 should be silent")
	}
	state.facts.Turn.ReadsLeft = 2
	if result := hook.On(state); result == nil || result.Hint == nil || result.Hint.Severity != "warning" {
		t.Fatalf("reads=2 result = %+v, want warning hint", result)
	}
	if result := hook.On(state); result != nil {
		t.Fatal("same quota warning should dedupe")
	}
	state.facts.Turn.ReadsLeft = 0
	if result := hook.On(state); result == nil || result.Hint == nil || result.Hint.Severity != "critical" {
		t.Fatalf("reads=0 result = %+v, want critical hint", result)
	}
}

func TestVerificationHook(t *testing.T) {
	hook := VerificationHook()
	state := newState()

	if result := hook.On(state); result != nil {
		t.Fatal("no tasks should be silent")
	}
	state.facts.Turn.HasTasks = true
	state.facts.Turn.TasksDone = true
	if result := hook.On(state); result != nil {
		t.Fatal("all tasks done should be silent")
	}
	state.facts.Turn.TasksDone = false
	if result := hook.On(state); result == nil || result.BlockFinal == nil || !strings.Contains(result.BlockFinal.Reason, "未完成") {
		t.Fatalf("unfinished no-tool result = %+v, want block final", result)
	}
	if result := hook.On(state); result != nil {
		t.Fatal("verification warning should dedupe")
	}
}

func TestVerificationHookUsesFinalIntent(t *testing.T) {
	hook := VerificationHook()
	state := newState()
	state.facts.Turn.HasTasks = true

	state.facts.Response.Intent = policy.FinalIntentFormatError
	if result := hook.On(state); result != nil {
		t.Fatalf("format error intent should be silent, got %+v", result)
	}
	state.facts.Response.Intent = policy.FinalIntentNonFinal
	if result := hook.On(state); result != nil {
		t.Fatalf("non-final intent should be silent, got %+v", result)
	}
	state.facts.Response.Intent = policy.FinalIntentFinal
	if result := hook.On(state); result == nil || result.BlockFinal == nil {
		t.Fatalf("final intent with unfinished tasks should block, got %+v", result)
	}
}

func TestGarbledCircuitBreaker(t *testing.T) {
	hook := GarbledCircuitBreaker()
	state := newState()
	state.facts.Response.GarbledCount = 4
	if result := hook.On(state); result != nil {
		t.Fatal("count=4 should not stop")
	}
	state.facts.Response.GarbledCount = 5
	if result := hook.On(state); result == nil || result.Stop == nil || *result.Stop != policy.StopFormatError {
		t.Fatalf("count=5 result = %+v, want format stop", result)
	}
}

func TestProgressStallHook(t *testing.T) {
	hook := ProgressStallHook()
	state := newState()
	state.facts.Turn.Input = "test task"
	state.facts.Activity.ToolCalls = 1

	for i := 0; i < 7; i++ {
		if result := hook.On(state); result != nil {
			t.Fatalf("stall turn %d result = %+v, want silent", i+1, result)
		}
	}
	if result := hook.On(state); result == nil || result.Hint == nil {
		t.Fatalf("8th stall result = %+v, want warning hint", result)
	} else if strings.Contains(result.Hint.Content, state.facts.Turn.Input) {
		t.Fatalf("policy hint elevated raw user input: %q", result.Hint.Content)
	}
}

func TestExplorationHooks(t *testing.T) {
	hook := ExplorationExhaustedHook()
	state := newState()
	state.facts.Turn.Input = "test task"
	state.facts.Activity.ExploreCalls = 10

	result := hook.On(state)
	if result == nil || result.Hint == nil || result.RequireTool != nil {
		t.Fatalf("exploration exhausted result = %+v, want hint", result)
	}
	if strings.Contains(result.Hint.Content, state.facts.Turn.Input) {
		t.Fatalf("policy hint elevated raw user input: %q", result.Hint.Content)
	}
}

func TestExploreCascadeHook(t *testing.T) {
	hook := ExploreCascadeHook()
	state := newState()
	state.facts.Turn.Input = "test task"

	state.facts.Activity.ResearcherCalls = 3
	if result := hook.On(state); result != nil {
		t.Fatal("3 researchers should be silent")
	}
	state.facts.Activity.ResearcherCalls = 4
	if result := hook.On(state); result == nil || result.Hint == nil || result.Hint.Type != "explore_cascade" {
		t.Fatalf("4 researchers result = %+v, want cascade hint", result)
	} else if strings.Contains(result.Hint.Content, state.facts.Turn.Input) {
		t.Fatalf("policy hint elevated raw user input: %q", result.Hint.Content)
	}
}

func TestToolResultGuardrailHook(t *testing.T) {
	hook := ToolResultGuardrailHook()
	state := newState()

	state.facts.Model.ToolResults = 40
	if result := hook.On(state); result != nil {
		t.Fatalf("threshold should be silent, got %+v", result)
	}
	state.facts.Model.ToolResults = 41
	if result := hook.On(state); result == nil || result.Hint == nil || result.Hint.Type != "tool_results" {
		t.Fatalf("tool result guardrail = %+v, want hint", result)
	}
	if state.Int("last_warned") != 41 {
		t.Fatalf("last warned = %d, want 41", state.Int("last_warned"))
	}
	state.facts.Model.ToolResults = 45
	if result := hook.On(state); result != nil {
		t.Fatalf("interval should dedupe, got %+v", result)
	}
}

func TestReadBeforeWriteHook(t *testing.T) {
	hook := ReadBeforeWriteHook()
	state := newState()
	state.facts.Tool = policy.ToolFacts{
		Name:         "edit",
		TargetPath:   "main.go",
		TargetExists: true,
	}

	if result := hook.On(state); result == nil || result.BlockTool == nil || !strings.Contains(result.BlockTool.Reason, "main.go") {
		t.Fatalf("unread edit result = %+v, want block", result)
	}
	state.facts.Tool.TargetWasRead = true
	if result := hook.On(state); result != nil {
		t.Fatalf("read target should pass, got %+v", result)
	}
	state.facts.Tool.TargetWasRead = false
	state.facts.Tool.EditAnchorSufficient = true
	if result := hook.On(state); result != nil {
		t.Fatalf("anchored edit should pass, got %+v", result)
	}
}

func TestReadOnlySpiralHook(t *testing.T) {
	hook := ReadOnlySpiralHook()
	state := newState()
	state.facts.Activity.ReadOnlyStreak = 2
	if result := hook.On(state); result != nil {
		t.Fatalf("streak=2 should be silent, got %+v", result)
	}
	state.facts.Activity.ReadOnlyStreak = 3
	if result := hook.On(state); result == nil || result.Hint == nil || result.Hint.Type != "read_only_spiral" {
		t.Fatalf("streak=3 result = %+v, want hint", result)
	}
	if result := hook.On(state); result != nil {
		t.Fatalf("same streak should dedupe, got %+v", result)
	}
}
