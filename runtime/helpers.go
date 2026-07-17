package runtime

import (
	"nekocode/runtime/internal/broker"
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/internal/recording"
	"nekocode/runtime/internal/redaction"
	"nekocode/runtime/internal/runstore"
)

type ApprovalBroker = broker.ApprovalBroker
type QuestionBroker = broker.QuestionBroker
type EventBus = eventbus.EventBus
type EventRecorder = recording.EventRecorder
type RunStore = runstore.RunStore
type Connector = connectors.Connector
type ConnectorStatusViewer = connectors.ConnectorStatusViewer
type ConnectorFactory = connectors.ConnectorFactory

func NewApprovalBroker(eventBus *EventBus, source SourceRef, runID func() RunID) *ApprovalBroker {
	return broker.NewApprovalBroker(eventBus, source, runID)
}

func NewQuestionBroker(eventBus *EventBus, source SourceRef, runID func() RunID) *QuestionBroker {
	return broker.NewQuestionBroker(eventBus, source, runID)
}

func NewEventBus() *EventBus {
	return eventbus.NewEventBus()
}

func NewDefaultEventRecorder() (*EventRecorder, error) {
	return recording.NewDefaultEventRecorder()
}

func NewEventRecorder(baseDir string) (*EventRecorder, error) {
	return recording.NewEventRecorder(baseDir)
}

func LoadRecordedEvents(baseDir string) ([]Event, error) {
	return recording.LoadRecordedEvents(baseDir)
}

func defaultEventRecorderBaseDir() string {
	return recording.DefaultBaseDir()
}

func NewRunStore(limit int) *RunStore {
	return runstore.NewRunStore(limit)
}

func RedactInputText(input string) string {
	return redaction.RedactInputText(input)
}
