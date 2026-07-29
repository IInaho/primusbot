package runtime

import "testing"

func TestStreamStateNilCallbacksAreNoOps(t *testing.T) {
	var s streamState
	// Must not panic with no callbacks wired.
	s.emitPhase("Thinking")
	s.emitText("hello")
	s.emitReasoning("hmm")
}

func TestStreamStateEmitPhase(t *testing.T) {
	var s streamState
	var got []string
	s.phase = func(phase string) { got = append(got, phase) }

	s.emitPhase("Thinking")
	s.emitPhase("Executing")

	if len(got) != 2 || got[0] != "Thinking" || got[1] != "Executing" {
		t.Fatalf("phases = %v, want [Thinking Executing]", got)
	}
}

func TestStreamStateEmitText(t *testing.T) {
	var s streamState
	var got string
	var isToolCall bool
	s.text = func(delta string, tool bool) { got += delta; isToolCall = tool }

	s.emitText("foo")
	s.emitText("bar")

	if got != "foobar" {
		t.Fatalf("text = %q, want %q", got, "foobar")
	}
	if isToolCall {
		t.Fatal("emitText should always report isToolCall=false")
	}
}

func TestStreamStateEmitReasoning(t *testing.T) {
	var s streamState
	var got string
	s.reasoning = func(delta string) { got += delta }

	s.emitReasoning("step 1. ")
	s.emitReasoning("step 2.")

	if got != "step 1. step 2." {
		t.Fatalf("reasoning = %q, want %q", got, "step 1. step 2.")
	}
}

func TestStreamStateResetReasoning(t *testing.T) {
	s := streamState{lastReason: "previous summary"}
	s.resetReasoning()
	if s.lastReason != "" {
		t.Fatalf("lastReason = %q, want empty after reset", s.lastReason)
	}
}
