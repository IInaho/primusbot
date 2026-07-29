package broker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/runtime/internal/eventbus"
)

type ApprovalBroker struct {
	mu       sync.Mutex
	nextID   uint64
	pending  map[string]*approvalRecord
	eventBus *eventbus.EventBus
	source   SourceRef
	runID    func() RunID
	closed   bool
}

type approvalRecord struct {
	view ApprovalView
	req  ConfirmRequest
}

type approvalHashInput struct {
	ToolName string      `json:"tool_name"`
	Kind     ConfirmKind `json:"kind"`
	ArgsHash string      `json:"args_hash"`
}

func NewApprovalBroker(eventBus *eventbus.EventBus, source SourceRef, runID func() RunID) *ApprovalBroker {
	return &ApprovalBroker{
		pending:  make(map[string]*approvalRecord),
		eventBus: eventBus,
		source:   source,
		runID:    runID,
	}
}

func (b *ApprovalBroker) Request(req ConfirmRequest) ConfirmReply {
	if req.Response == nil {
		req.Response = make(chan ConfirmReply, 1)
	}
	id := fmt.Sprintf("apr_%d", atomic.AddUint64(&b.nextID, 1))
	now := time.Now()
	argsHash := stableHash(req.Args)
	rec := &approvalRecord{
		req: req,
		view: ApprovalView{
			ID:                    id,
			ToolName:              req.ToolName,
			Args:                  req.Args,
			ArgsHash:              argsHash,
			ToolCallHash:          stableHash(approvalHashInput{ToolName: req.ToolName, Kind: req.Kind, ArgsHash: argsHash}),
			Kind:                  string(req.Kind),
			CanEscalatePermission: req.CanEscalatePermission,
			Status:                ApprovalPending,
			CreatedAt:             now,
			Source:                b.source,
		},
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ConfirmReply{Allowed: false}
	}
	b.pending[id] = rec
	viewCopy := rec.view
	b.mu.Unlock()

	b.publish(EventApprovalRequested, viewCopy)
	reply := <-req.Response
	if !req.CanEscalatePermission {
		reply.AllowWithPermission = false
	}

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

	reply := decision.ConfirmReply()
	if !viewCopy.CanEscalatePermission {
		reply.AllowWithPermission = false
	}
	rec.req.Response <- reply
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
	b.rejectAll(false)
}

func (b *ApprovalBroker) Close() {
	b.rejectAll(true)
}

func (b *ApprovalBroker) rejectAll(closeBroker bool) {
	b.mu.Lock()
	if closeBroker {
		b.closed = true
	}
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
		rec.req.Response <- ConfirmReply{Allowed: false}
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

func stableHash(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
