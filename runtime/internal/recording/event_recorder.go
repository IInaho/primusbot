package recording

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	commonview "nekocode/common/view"
	"nekocode/runtime/internal/artifact"
	"nekocode/runtime/internal/core"
	"nekocode/util/fs"
)

type RunID = core.RunID
type Event = core.Event
type EventType = core.EventType
type SourceRef = core.SourceRef
type MessagePayload = core.MessagePayload
type DeltaPayload = core.DeltaPayload
type PhasePayload = core.PhasePayload
type TodoItem = commonview.TodoItem
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload
type ApprovalView = core.ApprovalView
type QuestionView = core.QuestionView
type ConnectorStatusPayload = core.ConnectorStatusPayload

const (
	EventInputAccepted     = core.EventInputAccepted
	EventSystemMessage     = core.EventSystemMessage
	EventAssistantDelta    = core.EventAssistantDelta
	EventReasoningDelta    = core.EventReasoningDelta
	EventPhaseChanged      = core.EventPhaseChanged
	EventToolStarted       = core.EventToolStarted
	EventToolBlocked       = core.EventToolBlocked
	EventToolPreview       = core.EventToolPreview
	EventToolCompleted     = core.EventToolCompleted
	EventSubAgentStarted   = core.EventSubAgentStarted
	EventSubAgentEnded     = core.EventSubAgentEnded
	EventTodosUpdated      = core.EventTodosUpdated
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved
	EventRunDone           = core.EventRunDone
	EventRunFailed         = core.EventRunFailed
	EventRunAborted        = core.EventRunAborted
	EventConnectorStatus   = core.EventConnectorStatus
)

type EventRecorder struct {
	mu        sync.Mutex
	sessionID string
	baseDir   string
	runDir    string
	counts    map[RunID]int
}

type recordedEvent struct {
	ID          string          `json:"id"`
	RunID       RunID           `json:"run_id,omitempty"`
	Type        EventType       `json:"type"`
	Source      SourceRef       `json:"source"`
	Time        time.Time       `json:"time"`
	SessionID   string          `json:"session_id,omitempty"`
	PayloadType string          `json:"payload_type,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

func NewDefaultEventRecorder() (*EventRecorder, error) {
	return NewEventRecorder(DefaultBaseDir())
}

func DefaultBaseDir() string {
	return fs.NekocodeDataDir("runs")
}

func NewEventRecorder(baseDir string) (*EventRecorder, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, fmt.Errorf("runtime: empty event recorder base dir")
	}
	sessionID := time.Now().UTC().Format("20060102T150405.000000000Z")
	sessionID = strings.NewReplacer(":", "", ".", "-").Replace(sessionID)
	runDir := filepath.Join(baseDir, sessionID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return nil, err
	}
	return &EventRecorder{
		sessionID: sessionID,
		baseDir:   baseDir,
		runDir:    runDir,
		counts:    make(map[RunID]int),
	}, nil
}

func (r *EventRecorder) SessionID() string {
	if r == nil {
		return ""
	}
	return r.sessionID
}

func (r *EventRecorder) RunDir(runID RunID) string {
	if r == nil {
		return ""
	}
	return filepath.Join(r.runDir, safePathPart(string(runID)))
}

func (r *EventRecorder) Close() error {
	if r == nil {
		return nil
	}
	return nil
}

func (r *EventRecorder) Record(ev Event) {
	if err := r.RecordError(ev); err != nil {
		log.Printf("runtime: event recorder: %v", err)
	}
}

func (r *EventRecorder) RecordError(ev Event) error {
	if r == nil || ev.RunID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	runDir := r.RunDir(ev.RunID)
	if err := os.MkdirAll(filepath.Join(runDir, "artifacts"), 0o700); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	if err := appendJSONLine(filepath.Join(runDir, "events.jsonl"), recordedEventFrom(ev)); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	if err := r.recordArtifact(runDir, ev); err != nil {
		log.Printf("runtime: event recorder: artifact: %v", err)
	}
	return nil
}

func (r *EventRecorder) recordArtifact(runDir string, ev Event) error {
	switch ev.Type {
	case EventToolPreview:
		p, ok := ev.Payload.(ToolPayload)
		if !ok {
			return nil
		}
		return r.recordToolArtifact(runDir, ev.RunID, p.ToolName, p.Preview)
	case EventToolCompleted:
		p, ok := ev.Payload.(ToolPayload)
		if !ok || p.IsError {
			return nil
		}
		return r.recordToolArtifact(runDir, ev.RunID, p.ToolName, p.Output)
	case EventRunDone:
		p, ok := ev.Payload.(DonePayload)
		if !ok || strings.TrimSpace(p.Output) == "" {
			return nil
		}
		path := filepath.Join(runDir, "artifacts", "result.md")
		if err := fs.WriteFileWithDir(path, []byte(p.Output), 0o600); err != nil {
			return fmt.Errorf("write result.md: %w", err)
		}
	case EventRunFailed:
		p, ok := ev.Payload.(DonePayload)
		if !ok {
			return nil
		}
		if strings.TrimSpace(p.Output) != "" {
			if err := fs.WriteFileWithDir(filepath.Join(runDir, "artifacts", "partial-result.md"), []byte(p.Output), 0o600); err != nil {
				return fmt.Errorf("write partial-result.md: %w", err)
			}
		}
		if strings.TrimSpace(p.Error) != "" {
			if err := fs.WriteFileWithDir(filepath.Join(runDir, "artifacts", "error.txt"), []byte(p.Error), 0o600); err != nil {
				return fmt.Errorf("write error.txt: %w", err)
			}
		}
	}
	return nil
}

func (r *EventRecorder) recordToolArtifact(runDir string, runID RunID, toolName, content string) error {
	classification, ok := artifact.ClassifyToolOutput(toolName, content)
	if !ok {
		return nil
	}
	path := filepath.Join(runDir, "artifacts", r.nextArtifactName(runID, string(classification.Kind), classification.Extension))
	if err := fs.WriteFileWithDir(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write tool artifact: %w", err)
	}
	return nil
}

func (r *EventRecorder) nextArtifactName(runID RunID, kind, ext string) string {
	r.counts[runID]++
	return fmt.Sprintf("%s-%03d%s", kind, r.counts[runID], ext)
}

func recordedEventFrom(ev Event) recordedEvent {
	payload, payloadType := marshalPayload(ev.Payload)
	return recordedEvent{
		ID:          ev.ID,
		RunID:       ev.RunID,
		Type:        ev.Type,
		Source:      ev.Source,
		Time:        ev.Time,
		SessionID:   ev.SessionID,
		PayloadType: payloadType,
		Payload:     payload,
	}
}

func LoadRecordedEvents(baseDir string) ([]Event, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, fmt.Errorf("runtime: empty event recorder base dir")
	}
	matches, err := filepath.Glob(filepath.Join(baseDir, "*", "*", "events.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var out []Event
	for _, path := range matches {
		events, err := loadRecordedEventFile(path)
		if err != nil {
			log.Printf("runtime: skip recorded events %s: %v", path, err)
			continue
		}
		out = append(out, events...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out, nil
}

func loadRecordedEventFile(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec recordedEvent
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			log.Printf("runtime: recorded event %s:%d unmarshal: %v", path, lineNo, err)
			continue
		}
		ev, err := rec.Event()
		if err != nil {
			log.Printf("runtime: recorded event %s:%d decode: %v", path, lineNo, err)
			continue
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

func (r recordedEvent) Event() (Event, error) {
	payload, err := decodeRecordedPayload(r.Type, r.Payload)
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID:        r.ID,
		RunID:     r.RunID,
		Type:      r.Type,
		Source:    r.Source,
		Time:      r.Time,
		SessionID: r.SessionID,
		Payload:   payload,
	}, nil
}

func decodeRecordedPayload(typ EventType, data json.RawMessage) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	switch typ {
	case EventInputAccepted, EventSystemMessage:
		var p MessagePayload
		return p, json.Unmarshal(data, &p)
	case EventAssistantDelta, EventReasoningDelta:
		var p DeltaPayload
		return p, json.Unmarshal(data, &p)
	case EventPhaseChanged:
		var p PhasePayload
		return p, json.Unmarshal(data, &p)
	case EventToolStarted, EventToolBlocked, EventToolPreview, EventToolCompleted, EventSubAgentStarted, EventSubAgentEnded:
		var p ToolPayload
		return p, json.Unmarshal(data, &p)
	case EventTodosUpdated:
		var p []TodoItem
		return p, json.Unmarshal(data, &p)
	case EventApprovalRequested, EventApprovalResolved:
		var p ApprovalView
		return p, json.Unmarshal(data, &p)
	case EventQuestionRequested, EventQuestionResolved:
		var p QuestionView
		return p, json.Unmarshal(data, &p)
	case EventRunDone, EventRunFailed, EventRunAborted:
		var p DonePayload
		return p, json.Unmarshal(data, &p)
	case EventConnectorStatus:
		var p ConnectorStatusPayload
		return p, json.Unmarshal(data, &p)
	default:
		var p map[string]any
		return p, json.Unmarshal(data, &p)
	}
}

func marshalPayload(payload any) (json.RawMessage, string) {
	if payload == nil {
		return nil, ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		data, _ = json.Marshal(map[string]string{"error": err.Error()})
		return data, "marshal_error"
	}
	return data, payloadTypeName(payload)
}

func payloadTypeName(payload any) string {
	return strings.Replace(fmt.Sprintf("%T", payload), "core.", "runtime.", 1)
}

func appendJSONLine(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func safePathPart(s string) string {
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
