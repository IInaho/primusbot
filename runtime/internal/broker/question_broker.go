package broker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/runtime/internal/eventbus"
)

type QuestionBroker struct {
	mu       sync.Mutex
	nextID   uint64
	pending  map[string]*questionRecord
	eventBus *eventbus.EventBus
	source   SourceRef
	runID    func() RunID
}

type questionRecord struct {
	view QuestionView
	req  QuestionRequest
}

func NewQuestionBroker(eventBus *eventbus.EventBus, source SourceRef, runID func() RunID) *QuestionBroker {
	return &QuestionBroker{
		pending:  make(map[string]*questionRecord),
		eventBus: eventBus,
		source:   source,
		runID:    runID,
	}
}

func (b *QuestionBroker) Request(req QuestionRequest) QuestionReply {
	if req.Response == nil {
		req.Response = make(chan QuestionReply, 1)
	}
	id := fmt.Sprintf("q_%d", atomic.AddUint64(&b.nextID, 1))
	rec := &questionRecord{
		req: req,
		view: QuestionView{
			ID:        id,
			Questions: req.Questions,
			Status:    QuestionPending,
			CreatedAt: time.Now(),
			Source:    b.source,
		},
	}

	b.mu.Lock()
	b.pending[id] = rec
	viewCopy := rec.view
	b.mu.Unlock()

	b.publish(EventQuestionRequested, viewCopy)
	reply := <-req.Response

	b.mu.Lock()
	if current, ok := b.pending[id]; ok && current.view.Status == QuestionPending {
		resolvedAt := time.Now()
		current.view.ResolvedAt = &resolvedAt
		if reply.Rejected {
			current.view.Status = QuestionRejected
		} else {
			current.view.Status = QuestionAnswered
		}
		delete(b.pending, id)
		b.publish(EventQuestionResolved, current.view)
	}
	b.mu.Unlock()

	return reply
}

func (b *QuestionBroker) Answer(id string, reply QuestionReply) error {
	b.mu.Lock()
	rec, ok := b.pending[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("runtime: question %s not pending", id)
	}
	if rec.view.Status != QuestionPending {
		b.mu.Unlock()
		return fmt.Errorf("runtime: question %s already resolved", id)
	}
	resolvedAt := time.Now()
	rec.view.ResolvedAt = &resolvedAt
	if reply.Rejected {
		rec.view.Status = QuestionRejected
	} else {
		rec.view.Status = QuestionAnswered
	}
	delete(b.pending, id)
	viewCopy := rec.view
	b.mu.Unlock()

	rec.req.Response <- reply
	b.publish(EventQuestionResolved, viewCopy)
	return nil
}

func (b *QuestionBroker) Pending() []QuestionView {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]QuestionView, 0, len(b.pending))
	for _, rec := range b.pending {
		out = append(out, rec.view)
	}
	return out
}

func (b *QuestionBroker) RejectAll() {
	b.mu.Lock()
	records := make([]*questionRecord, 0, len(b.pending))
	for id, rec := range b.pending {
		resolvedAt := time.Now()
		rec.view.ResolvedAt = &resolvedAt
		rec.view.Status = QuestionRejected
		records = append(records, rec)
		delete(b.pending, id)
	}
	b.mu.Unlock()

	for _, rec := range records {
		rec.req.Response <- QuestionReply{Rejected: true}
		b.publish(EventQuestionResolved, rec.view)
	}
}

func (b *QuestionBroker) publish(typ EventType, payload any) {
	if b.eventBus == nil {
		return
	}
	b.eventBus.Publish(Event{
		RunID:   b.currentRunID(),
		Type:    typ,
		Source:  b.source,
		Payload: payload,
	})
}

func (b *QuestionBroker) currentRunID() RunID {
	if b.runID == nil {
		return ""
	}
	return b.runID()
}
