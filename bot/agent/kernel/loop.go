// Package kernel provides the generic agent loop skeleton shared by the
// main agent runtime and the subagent engine. It knows nothing about LLMs,
// tools, or hooks — only the control-flow shape of a step-driven loop.
package kernel

// Loop is the generic step-loop skeleton. Each callback is supplied by the
// caller; nil callbacks are treated as absent (Done and StepLimitReached
// report false, FinishStep and EvaluateStop are skipped).
type Loop struct {
	Done             func() bool
	StepLimitReached func() bool
	Step             func() bool
	FinishStep       func()
	EvaluateStop     func()
}

// RunLoop runs the loop until Done, the step limit is reached, or a step
// reports itself finished. EvaluateStop runs once after the loop exits.
func RunLoop(loop Loop) {
	for !loop.done() {
		if loop.stepLimitReached() {
			break
		}
		if loop.Step() {
			if loop.FinishStep != nil {
				loop.FinishStep()
			}
			break
		}
	}
	if loop.EvaluateStop != nil {
		loop.EvaluateStop()
	}
}

func (loop Loop) done() bool {
	return loop.Done != nil && loop.Done()
}

func (loop Loop) stepLimitReached() bool {
	return loop.StepLimitReached != nil && loop.StepLimitReached()
}
