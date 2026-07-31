package broker

import (
	"context"
	"testing"
	"time"

	"nekocode/protocol"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/eventbus"
)

func TestApprovalBrokerDecideUnblocksRequest(t *testing.T) {
	bus := eventbus.NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := bus.Subscribe(ctx, core.EventFilter{Types: []core.EventType{core.EventApprovalRequested}})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	broker := NewApprovalBroker(bus, core.SourceRef{Kind: "test"}, func() core.RunID { return "run_1" })
	result := make(chan protocol.ConfirmReply, 1)
	go func() {
		req := protocol.ConfirmRequest{
			ToolName: "shell",
			Args:     map[string]any{"command": "go test ./..."},
			Kind:     protocol.ConfirmKindPermission,
		}
		result <- broker.Request(req)
	}()

	var approval core.ApprovalView
	select {
	case ev := <-events:
		if ev.RunID != "run_1" {
			t.Fatalf("approval event run id = %q, want run_1", ev.RunID)
		}
		var ok bool
		approval, ok = ev.Payload.(core.ApprovalView)
		if !ok {
			t.Fatalf("payload type = %T, want core.ApprovalView", ev.Payload)
		}
		if approval.ArgsHash == "" || approval.ToolCallHash == "" {
			t.Fatalf("approval hashes were not set: %#v", approval)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for approval request")
	}

	if err := broker.Decide(approval.ID, core.ApprovalDecision{Allowed: true}); err != nil {
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

func TestApprovalBrokerHashesAreStable(t *testing.T) {
	first := stableHash(map[string]any{
		"command": "go test",
		"path":    "runtime",
	})
	second := stableHash(map[string]any{
		"path":    "runtime",
		"command": "go test",
	})
	if first == "" {
		t.Fatal("hash is empty")
	}
	if first != second {
		t.Fatalf("hashes differ for equivalent args: %q != %q", first, second)
	}
}

func TestApprovalBrokerRequestAfterCloseIsRejected(t *testing.T) {
	broker := NewApprovalBroker(nil, core.SourceRef{Kind: "test"}, nil)
	broker.Close()

	reply := broker.Request(protocol.ConfirmRequest{ToolName: "shell"})
	if reply.Allowed {
		t.Fatal("approval request was allowed after broker close")
	}
	if pending := broker.Pending(); len(pending) != 0 {
		t.Fatalf("pending approvals = %d, want 0", len(pending))
	}
}
