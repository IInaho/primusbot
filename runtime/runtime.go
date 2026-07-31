// Package runtime provides the interaction control layer above a Runner.
//
// Manager is the single in-process entry point used by TUI, GUI, HTTP and IM
// connectors. It owns run state, events, approvals, questions, connectors and
// read models; the runner only supplies bot execution and bot-owned views.
package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"nekocode/runtime/internal/broker"
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/internal/recording"
	"nekocode/runtime/internal/runstore"
)

type commandHandler func(ctx context.Context, args []string) (string, error)

// Interaction is the stable, transport-neutral contract shared by in-process
// UIs, HTTP adapters, and connectors. Optional capabilities remain discoverable
// through Manager instead of expanding this core boundary.
type Interaction interface {
	StartRun(context.Context, Input) (RunID, error)
	CancelRun(context.Context, RunID) error
	DecideApproval(context.Context, string, ApprovalDecision) error
	AnswerQuestion(context.Context, string, QuestionReply) error
	Events(context.Context, EventFilter) (<-chan Event, error)
}

// Manager is the runtime interaction kernel. Applications should keep this
// single instance and route every interaction surface through it.
type Manager struct {
	runner           Runner
	commander        Commander
	steerer          Steerer
	metrics          MetricsProvider
	models           ModelService
	context          ContextService
	extensions       ExtensionService
	configuration    ConfigurationService
	sessions         SessionService
	modelMutator     ModelMutator
	extensionMutator ExtensionMutator
	configMutator    ConfigurationMutator
	sessionMutator   SessionMutator
	closer           io.Closer
	events           *eventbus.EventBus
	approvals        *broker.ApprovalBroker
	questions        *broker.QuestionBroker
	connectors       *connectors.Manager
	runs             *runstore.RunStore
	recorder         *recording.EventRecorder
	runtimeCommands  map[string]commandHandler

	mu            sync.Mutex
	mutationMu    sync.Mutex
	recordingMu   sync.Mutex
	mutating      bool
	currentRun    RunID
	runContext    context.Context
	cancelRun     context.CancelFunc
	cancelDone    chan struct{}
	runDone       chan struct{}
	runLease      *runLease
	status        RunStatus
	latestMetrics MetricsSnapshot
	nextRun       uint64
	cancelled     map[RunID]struct{}
	closed        bool
	closeOnce     sync.Once
	closeErr      error
}

var (
	_ Interaction      = (*Manager)(nil)
	_ ConnectorRuntime = (*Manager)(nil)
)

// New constructs the interaction runtime around one runner. Every additional
// bot capability is discovered from the same instance.
func New(runner Runner) *Manager {
	if runner == nil {
		panic("runtime: nil runner")
	}
	events := eventbus.NewEventBus()
	rt := &Manager{
		runner:    runner,
		events:    events,
		runs:      runstore.NewRunStore(0),
		status:    RunIdle,
		cancelled: make(map[RunID]struct{}),
	}
	rt.commander, _ = runner.(Commander)
	rt.steerer, _ = runner.(Steerer)
	rt.metrics, _ = runner.(MetricsProvider)
	rt.models, _ = runner.(ModelService)
	rt.context, _ = runner.(ContextService)
	rt.extensions, _ = runner.(ExtensionService)
	rt.configuration, _ = runner.(ConfigurationService)
	rt.sessions, _ = runner.(SessionService)
	rt.modelMutator, _ = runner.(ModelMutator)
	rt.extensionMutator, _ = runner.(ExtensionMutator)
	rt.configMutator, _ = runner.(ConfigurationMutator)
	rt.sessionMutator, _ = runner.(SessionMutator)
	rt.closer, _ = runner.(io.Closer)
	events.AddObserver(rt.runs.Record)
	events.AddObserver(rt.recordMetrics)
	rt.approvals = broker.NewApprovalBroker(events, SourceRef{Kind: "runtime"}, rt.currentRunID)
	rt.questions = broker.NewQuestionBroker(events, SourceRef{Kind: "runtime"}, rt.currentRunID)
	rt.connectors = connectors.NewManager(rt)
	return rt
}

func (r *Manager) registerConnectorCommands() {
	r.registerCommand("connect", func(ctx context.Context, args []string) (string, error) {
		return r.connectors.Handle(ctx, args)
	})
	r.registerCommand("disconnect", func(ctx context.Context, args []string) (string, error) {
		connName := ""
		if len(args) > 0 {
			connName = args[0]
		}
		resp, err := r.connectors.Disconnect(connName)
		if err == nil && connName != "" {
			resp = ""
		}
		return resp, err
	})
	r.registerCommand("devices", func(_ context.Context, _ []string) (string, error) {
		return r.connectors.Devices(), nil
	})
}

// registerCommand adds a runtime-owned slash command.
func (r *Manager) registerCommand(name string, handler commandHandler) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runtimeCommands == nil {
		r.runtimeCommands = make(map[string]commandHandler)
	}
	r.runtimeCommands[name] = handler
}

func (r *Manager) currentRunID() RunID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentRun
}

func (r *Manager) ensureOpen() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return protocolError(ErrorClosed, "", "closed")
	}
	return nil
}

func (r *Manager) beginWaiting(runID RunID, status RunStatus) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.currentRun != runID || r.status != RunRunning {
		return false
	}
	if _, cancelled := r.cancelled[runID]; cancelled {
		return false
	}
	r.status = status
	return true
}

type cancelControl struct {
	runID     RunID
	cancel    context.CancelFunc
	lease     *runLease
	published chan struct{}
}

func (r *Manager) beginCancel(runID RunID) (cancelControl, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return cancelControl{}, false, protocolError(ErrorClosed, "cancel_run", "closed")
	}
	if runID == "" {
		runID = r.currentRun
	}
	if runID == "" || r.currentRun == "" {
		return cancelControl{}, false, nil
	}
	if runID != r.currentRun {
		return cancelControl{}, false, protocolError(ErrorConflict, "cancel_run", fmt.Sprintf("run %s is not active; current run is %s", runID, r.currentRun))
	}
	switch r.status {
	case RunRunning, RunWaitingApproval, RunWaitingQuestion:
	default:
		return cancelControl{runID: runID}, false, nil
	}
	r.cancelled[runID] = struct{}{}
	r.status = RunCancelled
	r.cancelDone = make(chan struct{})
	control := cancelControl{
		runID: runID, cancel: r.cancelRun, lease: r.runLease,
		published: r.cancelDone,
	}
	r.cancelRun = nil
	r.runLease = nil
	return control, true, nil
}

func (r *Manager) resumeRun(runID RunID, waitingStatus RunStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.currentRun != runID || r.status != waitingStatus {
		return
	}
	_, cancelled := r.cancelled[runID]
	if !cancelled {
		r.status = RunRunning
	}
}
