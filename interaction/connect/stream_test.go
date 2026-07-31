package connect

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStreamBufferMergesDeltas(t *testing.T) {
	s := &StreamBuffer{}
	now := time.Now()

	if chunk := s.Add("hello", now); chunk != "" {
		t.Fatalf("small delta should not flush, got %q", chunk)
	}
	if chunk := s.Add(" world", now.Add(100*time.Millisecond)); chunk != "" {
		t.Fatalf("deltas within interval should not flush, got %q", chunk)
	}
	if s.StreamedAny() {
		t.Fatal("nothing streamed yet")
	}

	chunk := s.Add("!", now.Add(3*time.Second))
	if chunk != "hello world!" {
		t.Fatalf("interval flush = %q, want merged text", chunk)
	}
	if !s.StreamedAny() {
		t.Fatal("streamed flag should be set after flush")
	}

	s.Add("tail", now.Add(4*time.Second))
	if got := s.Drain(); got != "tail" {
		t.Fatalf("drain = %q", got)
	}
	if got := s.Drain(); got != "" {
		t.Fatalf("second drain = %q, want empty", got)
	}

	s.Add("x", now)
	s.Reset()
	if got := s.Drain(); got != "" {
		t.Fatalf("after reset, drain = %q", got)
	}
	if s.StreamedAny() {
		t.Fatal("reset should clear streamed flag")
	}
}

func TestStreamBufferFlushesOnSize(t *testing.T) {
	s := &StreamBuffer{}
	big := strings.Repeat("a", StreamFlushRunes+10)
	if chunk := s.Add(big, time.Now()); len([]rune(chunk)) < StreamFlushRunes {
		t.Fatalf("size flush chunk too small: %d runes", len([]rune(chunk)))
	}
}

func TestBaseRunStateMachine(t *testing.T) {
	b := NewBase(nil, "test", "Test")

	ctx1, gen1 := b.Start(context.Background())
	if !b.IsRunning() {
		t.Fatal("should be running after Start")
	}

	// Second Start cancels the first run and bumps the generation.
	ctx2, gen2 := b.Start(context.Background())
	if gen2 != gen1+1 {
		t.Fatalf("generation = %d, want %d", gen2, gen1+1)
	}
	if ctx1.Err() == nil {
		t.Fatal("previous run context should be canceled")
	}
	_ = ctx2

	// Stale MarkStopped is a no-op.
	b.MarkStopped(gen1)
	if !b.IsRunning() {
		t.Fatal("stale MarkStopped should not clear running")
	}

	// Current MarkStopped clears; caller cancellation does not propagate
	// (WithoutCancel detach).
	caller, cancelCaller := context.WithCancel(context.Background())
	ctx3, gen3 := b.Start(caller)
	cancelCaller()
	if ctx3.Err() != nil {
		t.Fatal("run context must survive caller cancellation")
	}
	b.MarkStopped(gen3)
	if b.IsRunning() {
		t.Fatal("current MarkStopped should clear running")
	}

	// Stop on an idle base is a no-op safe call.
	ctx4, _ := b.Start(context.Background())
	b.Stop()
	if b.IsRunning() || ctx4.Err() == nil {
		t.Fatal("Stop should clear running and cancel the run context")
	}
}
