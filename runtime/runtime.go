// Package runtime provides the interaction control layer above a bot Backend.
//
// Manager is the single in-process entry point used by TUI, GUI, HTTP and IM
// connectors. It owns run state, events, approvals, questions, connectors and
// read models; the backend only supplies bot execution and bot-owned views.
package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"nekocode/runtime/internal/broker"
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/internal/recording"
	"nekocode/runtime/internal/runstore"
)

type CommandHandler func(ctx context.Context, args []string) (string, error)

// Manager is the runtime interaction kernel. Applications should keep this
// single instance and route every interaction surface through it.
type Manager struct {
	backend         Backend
	events          *eventbus.EventBus
	approvals       *broker.ApprovalBroker
	questions       *broker.QuestionBroker
	connectors      *connectors.Manager
	runs            *runstore.RunStore
	recorder        *recording.EventRecorder
	runtimeCommands map[string]CommandHandler

	mu         sync.Mutex
	currentRun RunID
	status     RunStatus
	nextRun    uint64
	aborted    map[RunID]struct{}
	closed     bool
	closeOnce  sync.Once
}

var (
	_ Control          = (*Manager)(nil)
	_ ConnectorRuntime = (*Manager)(nil)
	_ Client           = (*Manager)(nil)
)

// New constructs the interaction runtime around one bot backend.
func New(backend Backend) *Manager {
	if backend == nil {
		panic("runtime: nil backend")
	}
	events := eventbus.NewEventBus()
	rt := &Manager{
		backend: backend,
		events:  events,
		runs:    runstore.NewRunStore(0),
		status:  RunIdle,
		aborted: make(map[RunID]struct{}),
	}
	events.AddObserver(rt.runs.Record)
	rt.approvals = broker.NewApprovalBroker(events, SourceRef{Kind: "runtime"}, rt.currentRunID)
	rt.questions = broker.NewQuestionBroker(events, SourceRef{Kind: "runtime"}, rt.currentRunID)
	rt.connectors = connectors.NewManager(rt)
	rt.registerDefaultRuntimeCommands()
	rt.configureBackend()
	return rt
}

func (r *Manager) registerDefaultRuntimeCommands() {
	r.runtimeCommands = make(map[string]CommandHandler)
	r.RegisterCommand("connect", func(ctx context.Context, args []string) (string, error) {
		return r.connectors.Handle(ctx, args)
	})
	r.RegisterCommand("disconnect", func(ctx context.Context, args []string) (string, error) {
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
	r.RegisterCommand("devices", func(_ context.Context, _ []string) (string, error) {
		return r.connectors.Devices(), nil
	})
}

// RegisterCommand adds a runtime-owned slash command.
func (r *Manager) RegisterCommand(name string, handler CommandHandler) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || handler == nil {
		return
	}
	if r.runtimeCommands == nil {
		r.runtimeCommands = make(map[string]CommandHandler)
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
		return fmt.Errorf("runtime: closed")
	}
	return nil
}

func (r *Manager) beginWaiting(runID RunID, status RunStatus) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.currentRun != runID || r.status != RunRunning {
		return false
	}
	if _, aborted := r.aborted[runID]; aborted {
		return false
	}
	r.status = status
	return true
}

func (r *Manager) beginAbort(runID RunID) (RunID, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if runID == "" {
		runID = r.currentRun
	}
	if runID == "" || r.currentRun == "" {
		return "", false, nil
	}
	if runID != r.currentRun {
		return "", false, fmt.Errorf("runtime: run %s is not active; current run is %s", runID, r.currentRun)
	}
	switch r.status {
	case RunRunning, RunWaitingApproval, RunWaitingQuestion:
	default:
		return runID, false, nil
	}
	r.aborted[runID] = struct{}{}
	r.status = RunAborted
	return runID, true, nil
}

func (r *Manager) resumeRun(runID RunID, waitingStatus RunStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.currentRun != runID || r.status != waitingStatus {
		return
	}
	_, aborted := r.aborted[runID]
	if !aborted {
		r.status = RunRunning
	}
}
