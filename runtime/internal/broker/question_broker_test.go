package broker

import (
	"context"
	"testing"
	"time"

	"nekocode/protocol"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/eventbus"
)

func TestQuestionBrokerAnswerUnblocksRequest(t *testing.T) {
	bus := eventbus.NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := bus.Subscribe(ctx, core.EventFilter{Types: []core.EventType{core.EventQuestionRequested}})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	broker := NewQuestionBroker(bus, core.SourceRef{Kind: "test"}, func() core.RunID { return "run_1" })
	result := make(chan protocol.QuestionReply, 1)
	go func() {
		req := protocol.QuestionRequest{
			Questions: []protocol.QuestionItem{{Question: "Continue?"}},
		}
		result <- broker.Request(req)
	}()

	var question core.QuestionView
	select {
	case ev := <-events:
		if ev.RunID != "run_1" {
			t.Fatalf("question event run id = %q, want run_1", ev.RunID)
		}
		var ok bool
		question, ok = ev.Payload.(core.QuestionView)
		if !ok {
			t.Fatalf("payload type = %T, want core.QuestionView", ev.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for question request")
	}

	reply := protocol.QuestionReply{Answers: [][]string{{"yes"}}}
	if err := broker.Answer(question.ID, reply); err != nil {
		t.Fatalf("answer: %v", err)
	}

	select {
	case got := <-result:
		if got.Rejected || len(got.Answers) != 1 || got.Answers[0][0] != "yes" {
			t.Fatalf("unexpected reply: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker request to unblock")
	}
}

func TestQuestionBrokerRequestAfterCloseIsRejected(t *testing.T) {
	broker := NewQuestionBroker(nil, core.SourceRef{Kind: "test"}, nil)
	broker.Close()

	reply := broker.Request(protocol.QuestionRequest{
		Questions: []protocol.QuestionItem{{Question: "Continue?"}},
	})
	if !reply.Rejected {
		t.Fatal("question request was not rejected after broker close")
	}
	if pending := broker.Pending(); len(pending) != 0 {
		t.Fatalf("pending questions = %d, want 0", len(pending))
	}
}
