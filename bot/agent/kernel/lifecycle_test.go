package kernel

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleCancelAndReplace(t *testing.T) {
	l := NewLifecycle(context.Background(), 4)

	if l.Context().Err() != nil {
		t.Fatal("fresh context should not be canceled")
	}

	l.Cancel()
	if l.Context().Err() == nil {
		t.Fatal("after Cancel, context should be canceled")
	}

	l.ReplaceContext()
	if l.Context().Err() != nil {
		t.Fatal("after ReplaceContext, context should be live again")
	}
}

func TestLifecycleResetContextIfCanceled(t *testing.T) {
	l := NewLifecycle(context.Background(), 4)

	l.ResetContextIfCanceled()
	if l.Context().Err() != nil {
		t.Fatal("reset on live context should keep it live")
	}

	l.Cancel()
	l.ResetContextIfCanceled()
	if l.Context().Err() != nil {
		t.Fatal("reset on canceled context should revive it")
	}
}

func TestLifecycleReplaceContextKeepsParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	l := NewLifecycle(parent, 4)

	cancelParent()
	l.ReplaceContext()
	if l.Context().Err() == nil {
		t.Fatal("context derived from canceled parent should be canceled")
	}
}

func TestLifecycleSteeringBuffer(t *testing.T) {
	l := NewLifecycle(context.Background(), 2)

	for i := 0; i < 2; i++ {
		select {
		case l.Steering() <- "msg":
		default:
			t.Fatalf("send %d should fit in buffer", i)
		}
	}
	select {
	case l.Steering() <- "overflow":
		t.Fatal("third send should not fit in buffer of 2")
	default:
	}

	if got := <-l.Steering(); got != "msg" {
		t.Fatalf("drained %q, want msg", got)
	}
}

func TestLifecycleFinishedFlagAndDuration(t *testing.T) {
	l := NewLifecycle(context.Background(), 4)

	if d := l.Duration(); d != 0 {
		t.Fatalf("before Start, Duration = %v, want 0", d)
	}

	l.Finished().Store(true)
	l.Start()
	if l.Finished().Load() {
		t.Fatal("Start should clear the finished flag")
	}
	if d := l.Duration(); d <= 0 {
		t.Fatalf("after Start, Duration = %v, want > 0", d)
	}
	if l.startTime.After(time.Now()) {
		t.Fatal("start time should not be in the future")
	}
}
