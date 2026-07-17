package broker

import (
	"context"
	"testing"
	"time"

	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/view"
)

func TestApprovalBrokerDecideUnblocksRequest(t *testing.T) {
	bus := eventbus.NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := bus.Subscribe(ctx, core.EventFilter{Types: []core.EventType{core.EventApprovalRequested}})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	broker := NewApprovalBroker(bus, core.SourceRef{Kind: "test"}, func() RunID { return "run_1" })
	result := make(chan view.ConfirmReply, 1)
	go func() {
		req := view.NewConfirmRequest("shell", map[string]any{"command": "go test ./..."}, view.ConfirmKindPermission)
		result <- broker.Request(req)
	}()

	var approval ApprovalView
	select {
	case ev := <-events:
		if ev.RunID != "run_1" {
			t.Fatalf("approval event run id = %q, want run_1", ev.RunID)
		}
		var ok bool
		approval, ok = ev.Payload.(ApprovalView)
		if !ok {
			t.Fatalf("payload type = %T, want ApprovalView", ev.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for approval request")
	}

	if err := broker.Decide(approval.ID, ApprovalDecision{Allowed: true}); err != nil {
		t.Fatalf("decide: %v", err)
	}

	select {
	case got := <-result:
		if !got.Allowed {
			t.Fatal("approval reply was not allowed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker request to unblock")
	}
}
