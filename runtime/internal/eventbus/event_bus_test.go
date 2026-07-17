package eventbus

import (
	"context"
	"testing"
	"time"

	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/runstore"
)

func TestEventBusFiltersByRunAndType(t *testing.T) {
	bus := NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := bus.Subscribe(ctx, EventFilter{
		RunID: "run_1",
		Types: []EventType{core.EventRunDone},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	bus.Publish(Event{RunID: "run_2", Type: core.EventRunDone})
	bus.Publish(Event{RunID: "run_1", Type: core.EventRunStarted})
	want := bus.Publish(Event{RunID: "run_1", Type: core.EventRunDone})

	select {
	case got := <-events:
		if got.ID != want.ID {
			t.Fatalf("event id = %q, want %q", got.ID, want.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for filtered event")
	}
}

func TestEventBusSubscribeReplayIncludesHistoryAndLiveEvents(t *testing.T) {
	bus := NewEventBus()
	historical := bus.Publish(Event{RunID: "run_1", Type: core.EventSystemMessage, Payload: core.MessagePayload{Content: "old"}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := bus.SubscribeReplay(ctx, EventFilter{RunID: "run_1"})
	if err != nil {
		t.Fatalf("SubscribeReplay: %v", err)
	}

	live := bus.Publish(Event{RunID: "run_1", Type: core.EventRunDone, Payload: core.DonePayload{Output: "new"}})

	gotHistorical := <-events
	if gotHistorical.ID != historical.ID {
		t.Fatalf("first replay id = %q, want %q", gotHistorical.ID, historical.ID)
	}
	gotLive := <-events
	if gotLive.ID != live.ID {
		t.Fatalf("live id = %q, want %q", gotLive.ID, live.ID)
	}
}

func TestEventBusImportHistoryAdvancesEventID(t *testing.T) {
	bus := NewEventBus()
	bus.ImportHistory([]Event{{ID: "evt_41", RunID: "run_1", Type: core.EventRunStarted}})
	ev := bus.Publish(Event{RunID: "run_1", Type: core.EventRunDone})
	if ev.ID != "evt_42" {
		t.Fatalf("event id after import = %q, want evt_42", ev.ID)
	}
}

func TestEventBusDoesNotDropApprovalResolvedWhenSubscriberBacklogged(t *testing.T) {
	bus := NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := bus.Subscribe(ctx, EventFilter{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for range 256 {
		bus.Publish(Event{RunID: "run_1", Type: core.EventToolStarted})
	}
	resolved := bus.Publish(Event{RunID: "run_1", Type: core.EventApprovalResolved})

	for {
		select {
		case got := <-events:
			if got.ID == resolved.ID {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("approval_resolved was dropped from a backlogged subscriber")
		}
	}
}

func TestEventBusObserverReceivesPublishedEvent(t *testing.T) {
	bus := NewEventBus()
	store := runstore.NewRunStore(0)
	bus.AddObserver(store.Record)

	bus.Publish(Event{
		RunID:   "run_1",
		Type:    core.EventRunDone,
		Payload: core.DonePayload{Output: "ok"},
	})

	view, ok := store.RunView("run_1")
	if !ok {
		t.Fatal("observer did not record event")
	}
	if view.Status != core.RunDone || view.Output != "ok" {
		t.Fatalf("view = %#v", view)
	}
}
