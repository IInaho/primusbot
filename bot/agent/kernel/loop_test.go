package kernel

import "testing"

func TestRunLoopRunsUntilStepFinishes(t *testing.T) {
	var steps, finishes, stops int
	RunLoop(Loop{
		Step: func() bool {
			steps++
			return steps == 3 // finish on the third step
		},
		FinishStep:   func() { finishes++ },
		EvaluateStop: func() { stops++ },
	})

	if steps != 3 {
		t.Fatalf("steps = %d, want 3", steps)
	}
	if finishes != 1 {
		t.Fatalf("FinishStep called %d times, want 1", finishes)
	}
	if stops != 1 {
		t.Fatalf("EvaluateStop called %d times, want 1", stops)
	}
}

func TestRunLoopStopsWhenDone(t *testing.T) {
	var steps int
	RunLoop(Loop{
		Done:         func() bool { return true },
		Step:         func() bool { steps++; return false },
		EvaluateStop: func() {},
	})
	if steps != 0 {
		t.Fatalf("steps = %d, want 0 when Done is true before the loop", steps)
	}
}

func TestRunLoopStopsAtStepLimit(t *testing.T) {
	var steps, finishes int
	RunLoop(Loop{
		StepLimitReached: func() bool { return steps >= 5 },
		Step:             func() bool { steps++; return false },
		FinishStep:       func() { finishes++ },
	})
	if steps != 5 {
		t.Fatalf("steps = %d, want 5", steps)
	}
	if finishes != 0 {
		t.Fatalf("FinishStep called %d times, want 0 (limit break is not a finished step)", finishes)
	}
}

func TestRunLoopNilCallbacksAreSafe(t *testing.T) {
	// Only Step is required; everything else nil must not panic, and the
	// loop must terminate when Step reports finished.
	steps := 0
	RunLoop(Loop{Step: func() bool { steps++; return true }})
	if steps != 1 {
		t.Fatalf("steps = %d, want 1", steps)
	}
}

func TestRunLoopEvaluateStopRunsOnEveryExitPath(t *testing.T) {
	paths := map[string]Loop{
		"done": {
			Done: func() bool { return true },
			Step: func() bool { return false },
		},
		"limit": {
			StepLimitReached: func() bool { return true },
			Step:             func() bool { return false },
		},
		"finished": {
			Step: func() bool { return true },
		},
	}
	for name, loop := range paths {
		stops := 0
		loop.EvaluateStop = func() { stops++ }
		RunLoop(loop)
		if stops != 1 {
			t.Errorf("%s exit: EvaluateStop called %d times, want 1", name, stops)
		}
	}
}
