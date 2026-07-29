package core

import (
	"strings"
	"sync"
	"time"
)

// StreamBuffer accumulates assistant/reasoning deltas and emits merged
// chunks, so high-frequency streaming events do not flood a chat with one
// message per token. Channels without their own stream tracker (e.g.
// telegram's taskview) can use it directly.
type StreamBuffer struct {
	mu        sync.Mutex
	buf       strings.Builder
	lastFlush time.Time
	streamed  bool
}

const (
	// StreamFlushRunes is the pending-text size that forces a flush.
	StreamFlushRunes = 800
	// StreamFlushInterval is the idle time after which pending text flushes.
	StreamFlushInterval = 2 * time.Second
)

// Add appends a delta and returns the chunk to send when the flush
// threshold (size or interval) is reached, "" otherwise.
func (s *StreamBuffer) Add(delta string, now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.WriteString(delta)
	// The flush interval starts at the first delta, not at the zero time.
	if s.lastFlush.IsZero() {
		s.lastFlush = now
	}
	pending := len([]rune(s.buf.String()))
	if pending < StreamFlushRunes && now.Sub(s.lastFlush) < StreamFlushInterval {
		return ""
	}
	return s.drainLocked(now)
}

// Drain returns and clears the pending text (e.g. at run end).
func (s *StreamBuffer) Drain() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.drainLocked(time.Now())
}

// StreamedAny reports whether any chunk was delivered since the last Reset.
func (s *StreamBuffer) StreamedAny() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamed
}

// Reset clears the buffer and the streamed flag for a new run.
func (s *StreamBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Reset()
	s.streamed = false
	s.lastFlush = time.Time{}
}

func (s *StreamBuffer) drainLocked(now time.Time) string {
	chunk := s.buf.String()
	if chunk == "" {
		return ""
	}
	s.buf.Reset()
	s.streamed = true
	s.lastFlush = now
	return chunk
}
