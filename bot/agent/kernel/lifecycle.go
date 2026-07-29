package kernel

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Lifecycle is the interruptible, revivable execution context of an agent
// run: Cancel aborts the current context, ReplaceContext derives a fresh one
// from the parent (used to retry after interruption), and the steering
// mailbox lets callers inject messages while a run is in flight.
type Lifecycle struct {
	mu        sync.Mutex
	parentCtx context.Context
	curCtx    context.Context
	cancelFn  context.CancelFunc
	steering  chan string
	startTime time.Time
	finished  atomic.Bool
}

// NewLifecycle derives the initial context from parent and creates a
// steering mailbox with the given buffer size.
func NewLifecycle(parent context.Context, steeringBuf int) *Lifecycle {
	ctx, cancel := context.WithCancel(parent)
	return &Lifecycle{
		parentCtx: parent,
		curCtx:    ctx,
		cancelFn:  cancel,
		steering:  make(chan string, steeringBuf),
	}
}

// Context returns the current run context.
func (l *Lifecycle) Context() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.curCtx
}

// ReplaceContext cancels the current context and derives a fresh one from
// the parent.
func (l *Lifecycle) ReplaceContext() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancelFn()
	l.curCtx, l.cancelFn = context.WithCancel(l.parentCtx)
}

// ResetContextIfCanceled derives a fresh context from the parent only when
// the current one is already canceled.
func (l *Lifecycle) ResetContextIfCanceled() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.curCtx.Err() != nil {
		l.curCtx, l.cancelFn = context.WithCancel(l.parentCtx)
	}
}

// Cancel aborts the current context.
func (l *Lifecycle) Cancel() {
	l.mu.Lock()
	l.cancelFn()
	l.mu.Unlock()
}

// Duration reports how long the run has been going, or 0 before Start.
func (l *Lifecycle) Duration() time.Duration {
	if l.startTime.IsZero() {
		return 0
	}
	return time.Since(l.startTime)
}

// Start marks the run as unfinished and records its start time.
func (l *Lifecycle) Start() {
	l.finished.Store(false)
	l.startTime = time.Now()
}

// Finished exposes the run's finished flag for atomic Load/Store by callers.
func (l *Lifecycle) Finished() *atomic.Bool {
	return &l.finished
}

// Steering returns the steering mailbox: senders inject messages, the run
// loop drains them.
func (l *Lifecycle) Steering() chan string {
	return l.steering
}
