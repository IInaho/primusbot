package builtin

import (
	"fmt"
	"strings"
	"testing"
)

// Every builtin hook must describe its own trigger context so the registry
// does not need per-hook knowledge for audit output.
func TestAllHooksDescribeTrigger(t *testing.T) {
	for _, h := range All() {
		if h.DescribeTrigger == nil {
			t.Errorf("hook %q has no DescribeTrigger", h.Name)
		}
	}
}

// The guardrail's audit text must stay in lockstep with its threshold
// constants (single source of truth).
func TestToolResultGuardrailTriggerUsesConstants(t *testing.T) {
	hk := ToolResultGuardrailHook()
	s := newState()
	s.facts.Model.ToolResults = 55
	s.SetInt("last_warned", 40)

	got := hk.DescribeTrigger(s)
	want := fmt.Sprintf("tool_results=55 last_warned=40 threshold=%d interval=%d",
		toolResultThreshold, toolResultInterval)
	if got != want {
		t.Fatalf("trigger = %q, want %q", got, want)
	}
}

func TestReadBeforeWriteTriggerDashForMissingTarget(t *testing.T) {
	hk := ReadBeforeWriteHook()
	s := newState()
	got := hk.DescribeTrigger(s)
	if !strings.HasPrefix(got, "target=- ") {
		t.Fatalf("trigger = %q, want target=- for missing path", got)
	}
}
