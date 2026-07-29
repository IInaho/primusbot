package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nekocode/runtime/internal/recording"
)

func (r *Manager) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.Subscribe(ctx, filter)
}

func (r *Manager) SubscribeReplay(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.SubscribeReplay(ctx, filter)
}

// ReportConnectorStatus publishes the only event type connectors may inject.
func (r *Manager) ReportConnectorStatus(payload ConnectorStatusPayload) {
	if payload.Name == "" {
		return
	}
	r.events.Publish(Event{
		Type:    EventConnectorStatus,
		Source:  SourceRef{Kind: payload.Name},
		Payload: payload,
	})
}

func (r *Manager) EventHistory(filter EventFilter) []Event {
	return r.events.History(filter)
}

// EnableDefaultEventRecording persists run events under the NekoCode data
// directory and restores previously recorded run history.
func (r *Manager) EnableDefaultEventRecording() error {
	return r.EnableEventRecording(recording.DefaultBaseDir())
}

// EnableEventRecording persists run events under baseDir and restores
// previously recorded run history.
func (r *Manager) EnableEventRecording(baseDir string) error {
	if r.recorder != nil {
		return nil
	}
	if strings.TrimSpace(baseDir) == "" {
		return fmt.Errorf("runtime: empty event recording directory")
	}
	events, err := recording.LoadRecordedEvents(baseDir)
	if err != nil {
		return fmt.Errorf("runtime: load recorded events: %w", err)
	}
	if len(events) > 0 {
		for _, ev := range events {
			r.runs.Record(ev)
		}
		r.events.ImportHistory(events)
		r.advanceRunSequence(events)
	}
	recorder, err := recording.NewEventRecorder(baseDir)
	if err != nil {
		return err
	}
	r.recorder = recorder
	r.events.AddObserver(recorder.Record)
	return nil
}

func (r *Manager) advanceRunSequence(events []Event) {
	var maxID uint64
	for _, ev := range events {
		n, ok := parseRunSequence(ev.RunID)
		if ok && n > maxID {
			maxID = n
		}
	}
	if maxID == 0 {
		return
	}
	r.mu.Lock()
	if maxID > r.nextRun {
		r.nextRun = maxID
	}
	r.mu.Unlock()
}

func parseRunSequence(runID RunID) (uint64, bool) {
	raw := strings.TrimPrefix(string(runID), "run_")
	if raw == string(runID) || raw == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	return n, err == nil
}
