package builtin

import (
	"testing"

	"nekocode/bot/policy"
)

func TestRegisterRegistersExpectedHookSet(t *testing.T) {
	r := policy.NewRegistry()
	Register(r)

	hooks := r.List()
	if len(hooks) != 9 {
		t.Fatalf("builtin hooks = %d, want 9", len(hooks))
	}
	want := map[string]bool{
		"quota":                   true,
		"tool_result_guardrail":   true,
		"read_before_write":       true,
		"read_only_spiral":        true,
		"verification":            true,
		"exploration_exhausted":   true,
		"explore_cascade":         true,
		"progress_stall":          true,
		"garbled_circuit_breaker": true,
	}
	for _, h := range hooks {
		if !want[h.Name] {
			t.Fatalf("unexpected builtin hook %q", h.Name)
		}
	}
}
