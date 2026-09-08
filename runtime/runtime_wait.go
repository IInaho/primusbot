package runtime

import "context"

// WaitRun waits until the identified run has completed all lifecycle cleanup.
// It is safe to call after receiving a terminal run event.
func (r *Runtime) WaitRun(ctx context.Context, runID RunID) error {
	r.mu.Lock()
	if runID == "" || r.currentRun != runID || r.status == RunIdle {
		r.mu.Unlock()
		return nil
	}
	done := r.runDone
	r.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
