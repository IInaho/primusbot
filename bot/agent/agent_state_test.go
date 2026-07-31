package agent

import (
	"sync"
	"testing"
)

func TestStreamStateNilCallbacksAreNoOps(t *testing.T) {
	var s streamState

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
	s.text = func(delta string) { got += delta }

	s.emitText("foo")
	s.emitText("bar")

	if got != "foobar" {
		t.Fatalf("text = %q, want %q", got, "foobar")
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

func TestTokenMeterZeroValue(t *testing.T) {
	var m tokenMeter
	prompt, completion := m.total(100)
	if prompt != 100 || completion != 0 {
		t.Fatalf("total() = (%d, %d), want (100, 0)", prompt, completion)
	}
	prompt, completion = m.turn(100)
	if prompt != 100 || completion != 0 {
		t.Fatalf("turn() before snapshot = (%d, %d), want (100, 0)", prompt, completion)
	}
}

func TestTokenMeterAccumulatesCompletion(t *testing.T) {
	var m tokenMeter
	m.add(10, 3)
	m.add(20, 4)

	prompt, completion := m.total(500)
	if prompt != 500 {
		t.Fatalf("total() prompt = %d, want 500 (live context tokens)", prompt)
	}
	if completion != 7 {
		t.Fatalf("total() completion = %d, want 7", completion)
	}
}

func TestTokenMeterTurnReportsDeltaSinceSnapshot(t *testing.T) {
	var m tokenMeter
	m.add(0, 5)
	m.snapshot(1000)
	m.add(0, 8)

	prompt, completion := m.turn(1200)
	if prompt != 200 {
		t.Fatalf("turn() prompt = %d, want 200 (1200-1000)", prompt)
	}
	if completion != 8 {
		t.Fatalf("turn() completion = %d, want 8 (only post-snapshot tokens)", completion)
	}

	m.snapshot(1200)
	m.add(0, 2)
	prompt, completion = m.turn(1500)
	if prompt != 300 || completion != 2 {
		t.Fatalf("turn() after re-snapshot = (%d, %d), want (300, 2)", prompt, completion)
	}
}

func TestTokenMeterConcurrentAdds(t *testing.T) {
	var m tokenMeter
	const workers = 8
	const perWorker = 1000

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				m.add(1, 2)
			}
		}()
	}
	wg.Wait()

	_, completion := m.total(0)
	if want := workers * perWorker * 2; completion != want {
		t.Fatalf("completion = %d, want %d", completion, want)
	}
}
