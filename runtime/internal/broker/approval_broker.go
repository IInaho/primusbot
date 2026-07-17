package broker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/view"
)

type ApprovalBroker struct {
	mu       sync.Mutex
	nextID   uint64
	pending  map[string]*approvalRecord
	eventBus *eventbus.EventBus
	source   SourceRef
	runID    func() RunID
}

type approvalRecord struct {
	view ApprovalView
	req  view.ConfirmRequest
}

func NewApprovalBroker(eventBus *eventbus.EventBus, source SourceRef, runID func() RunID) *ApprovalBroker {
	return &ApprovalBroker{
		pending:  make(map[string]*approvalRecord),
		eventBus: eventBus,
		source:   source,
		runID:    runID,
	}
}

func (b *ApprovalBroker) Request(req view.ConfirmRequest) view.ConfirmReply {
	if req.Response == nil {
		req.Response = make(chan view.ConfirmReply, 1)
	}
	id := fmt.Sprintf("apr_%d", atomic.AddUint64(&b.nextID, 1))
	now := time.Now()
	rec := &approvalRecord{
		req: req,
		view: ApprovalView{
			ID:                    id,
			ToolName:              req.ToolName,
			Args:                  req.Args,
			Kind:                  string(req.Kind),
			CanEscalatePermission: req.CanEscalatePermission,
			Status:                ApprovalPending,
			CreatedAt:             now,
			Source:                b.source,
		},
	}

	b.mu.Lock()
	b.pending[id] = rec
	b.mu.Unlock()

	b.publish(EventApprovalRequested, rec.view)
	reply := <-req.Response

	b.mu.Lock()
	if current, ok := b.pending[id]; ok && current.view.Status == ApprovalPending {
		resolvedAt := time.Now()
		current.view.ResolvedAt = &resolvedAt
		if reply.Allowed {
			current.view.Status = ApprovalApproved
		} else {
			current.view.Status = ApprovalRejected
		}
		delete(b.pending, id)
		b.publish(EventApprovalResolved, current.view)
	}
	b.mu.Unlock()

	return reply
}

func (b *ApprovalBroker) Decide(id string, decision ApprovalDecision) error {
	b.mu.Lock()
	rec, ok := b.pending[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("runtime: approval %s not pending", id)
	}
	if rec.view.Status != ApprovalPending {
		b.mu.Unlock()
		return fmt.Errorf("runtime: approval %s already resolved", id)
	}
	resolvedAt := time.Now()
	rec.view.ResolvedAt = &resolvedAt
	if decision.Allowed {
		rec.view.Status = ApprovalApproved
	} else {
		rec.view.Status = ApprovalRejected
	}
	delete(b.pending, id)
	viewCopy := rec.view
	b.mu.Unlock()

	rec.req.Response <- decision.ConfirmReply()
	b.publish(EventApprovalResolved, viewCopy)
	return nil
}

func (b *ApprovalBroker) Pending() []ApprovalView {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]ApprovalView, 0, len(b.pending))
	for _, rec := range b.pending {
		out = append(out, rec.view)
	}
	return out
}

func (b *ApprovalBroker) RejectAll() {
	b.mu.Lock()
	records := make([]*approvalRecord, 0, len(b.pending))
	for id, rec := range b.pending {
		resolvedAt := time.Now()
		rec.view.ResolvedAt = &resolvedAt
		rec.view.Status = ApprovalRejected
		records = append(records, rec)
		delete(b.pending, id)
	}
	b.mu.Unlock()

	for _, rec := range records {
		rec.req.Response <- view.ConfirmReply{Allowed: false}
		b.publish(EventApprovalResolved, rec.view)
	}
}

func (b *ApprovalBroker) publish(typ EventType, payload any) {
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

func (b *ApprovalBroker) currentRunID() RunID {
	if b.runID == nil {
		return ""
	}
	return b.runID()
}
