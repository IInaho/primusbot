package eventbus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/runtime/internal/core"
)

type Event = core.Event
type EventFilter = core.EventFilter
type EventType = core.EventType

const (
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved
	EventRunDone           = core.EventRunDone
	EventRunFailed         = core.EventRunFailed
	EventRunAborted        = core.EventRunAborted
)

const defaultSubscriberBuffer = 128
const defaultEventHistoryLimit = 1000

type EventBus struct {
	mu           sync.Mutex
	nextSubID    int
	nextEventID  uint64
	subscribers  map[int]subscriber
	observers    []func(Event)
	history      []Event
	historyLimit int
	closed       bool
}

type subscriber struct {
	filter EventFilter
	ch     chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[int]subscriber), historyLimit: defaultEventHistoryLimit}
}

func (b *EventBus) AddObserver(fn func(Event)) {
	if b == nil || fn == nil {
		return
	}
	b.mu.Lock()
	b.observers = append(b.observers, fn)
	b.mu.Unlock()
}

func (b *EventBus) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return b.subscribe(ctx, filter, false)
}

func (b *EventBus) SubscribeReplay(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return b.subscribe(ctx, filter, true)
}

func (b *EventBus) subscribe(ctx context.Context, filter EventFilter, replay bool) (<-chan Event, error) {
	if b == nil {
		return nil, fmt.Errorf("runtime: nil event bus")
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, fmt.Errorf("runtime: event bus closed")
	}
	var history []Event
	if replay {
		for _, ev := range b.history {
			if eventMatches(filter, ev) {
				history = append(history, ev)
			}
		}
	}
	b.nextSubID++
	id := b.nextSubID
	ch := make(chan Event, defaultSubscriberBuffer+len(history))
	for _, ev := range history {
		ch <- ev
	}
	b.subscribers[id] = subscriber{filter: filter, ch: ch}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if sub, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(sub.ch)
		}
		b.mu.Unlock()
	}()

	return ch, nil
}

func (b *EventBus) Publish(ev Event) Event {
	if b == nil {
		return ev
	}
	if ev.ID == "" {
		ev.ID = fmt.Sprintf("evt_%d", atomic.AddUint64(&b.nextEventID, 1))
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ev
	}
	b.history = append(b.history, ev)
	if b.historyLimit > 0 && len(b.history) > b.historyLimit {
		b.history = append([]Event(nil), b.history[len(b.history)-b.historyLimit:]...)
	}
	for _, sub := range b.subscribers {
		if eventMatches(sub.filter, ev) {
			deliverEvent(sub.ch, ev)
		}
	}
	observers := append([]func(Event){}, b.observers...)
	b.mu.Unlock()

	for _, observer := range observers {
		observer(ev)
	}
	return ev
}

func deliverEvent(ch chan Event, ev Event) {
	select {
	case ch <- ev:
		return
	default:
	}
	if !mustDeliverEvent(ev.Type) {
		return
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- ev:
	default:
	}
}

func mustDeliverEvent(typ EventType) bool {
	switch typ {
	case EventApprovalRequested,
		EventApprovalResolved,
		EventQuestionRequested,
		EventQuestionResolved,
		EventRunDone,
		EventRunFailed,
		EventRunAborted:
		return true
	default:
		return false
	}
}

func (b *EventBus) History(filter EventFilter) []Event {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, 0, len(b.history))
	for _, ev := range b.history {
		if eventMatches(filter, ev) {
			out = append(out, ev)
		}
	}
	return out
}

func (b *EventBus) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	subscribers := b.subscribers
	b.subscribers = nil
	b.observers = nil
	b.mu.Unlock()
	for _, sub := range subscribers {
		close(sub.ch)
	}
}

func (b *EventBus) ImportHistory(events []Event) {
	if b == nil || len(events) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.history = append(b.history, events...)
	var maxID uint64
	for _, ev := range b.history {
		if n, ok := parseEventSequence(ev.ID); ok && n > maxID {
			maxID = n
		}
	}
	if maxID > atomic.LoadUint64(&b.nextEventID) {
		atomic.StoreUint64(&b.nextEventID, maxID)
	}
	if b.historyLimit > 0 && len(b.history) > b.historyLimit {
		b.history = append([]Event(nil), b.history[len(b.history)-b.historyLimit:]...)
	}
}

func parseEventSequence(id string) (uint64, bool) {
	raw := strings.TrimPrefix(id, "evt_")
	if raw == id || raw == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	return n, err == nil
}

func eventMatches(filter EventFilter, ev Event) bool {
	if filter.RunID != "" && filter.RunID != ev.RunID {
		return false
	}
	if len(filter.Types) > 0 {
		found := false
		for _, typ := range filter.Types {
			if typ == ev.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Sources) > 0 {
		found := false
		for _, src := range filter.Sources {
			if src == ev.Source.Kind {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
