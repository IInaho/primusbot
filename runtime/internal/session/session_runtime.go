package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"nekocode/runtime/internal/connectors"
)

type RuntimeCommandHandler func(ctx context.Context, rt *SessionRuntime, args []string) (string, error)

type SessionRuntime struct {
	runner          AgentRunner
	commands        CommandExecutor
	skills          SkillHintProvider
	catalog         CommandCatalog
	control         RunController
	stats           StatsProvider
	model           ModelInfoProvider
	messages        MessageProvider
	events          *EventBus
	approvals       *ApprovalBroker
	questions       *QuestionBroker
	connectors      *connectors.Manager
	runs            *RunStore
	recorder        *EventRecorder
	runtimeCommands map[string]RuntimeCommandHandler

	mu          sync.Mutex
	currentRun  RunID
	status      RunStatus
	nextRun     uint64
	aborted     map[RunID]struct{}
	confirmDone chan struct{}
	closeOnce   sync.Once
}

type Ports struct {
	Runner   AgentRunner
	Commands CommandExecutor
	Skills   SkillHintProvider
	Catalog  CommandCatalog
	Control  RunController
	Stats    StatsProvider
	Model    ModelInfoProvider
	Messages MessageProvider
}

func NewSessionRuntimeWithPorts(ports Ports) *SessionRuntime {
	ports = fillDefaultPorts(ports)
	events := NewEventBus()
	rt := &SessionRuntime{
		runner:   ports.Runner,
		commands: ports.Commands,
		skills:   ports.Skills,
		catalog:  ports.Catalog,
		control:  ports.Control,
		stats:    ports.Stats,
		model:    ports.Model,
		messages: ports.Messages,
		events:   events,
		runs:     NewRunStore(0),
		status:   RunIdle,
		aborted:  make(map[RunID]struct{}),
	}
	events.AddObserver(rt.runs.Record)
	rt.approvals = NewApprovalBroker(events, SourceRef{Kind: "runtime"}, rt.CurrentRunID)
	rt.questions = NewQuestionBroker(events, SourceRef{Kind: "runtime"}, rt.CurrentRunID)
	rt.connectors = connectors.NewManager(rt)
	rt.registerDefaultRuntimeCommands()
	rt.configureBot()
	return rt
}

func (r *SessionRuntime) registerDefaultRuntimeCommands() {
	r.runtimeCommands = make(map[string]RuntimeCommandHandler)
	r.RegisterRuntimeCommand("connect", func(ctx context.Context, rt *SessionRuntime, args []string) (string, error) {
		return rt.connectors.Handle(ctx, args)
	})
	r.RegisterRuntimeCommand("disconnect", func(ctx context.Context, rt *SessionRuntime, args []string) (string, error) {
		connName := ""
		if len(args) > 0 {
			connName = args[0]
		}
		resp, err := rt.connectors.Disconnect(connName)
		if err == nil && connName != "" {
			resp = ""
		}
		return resp, err
	})
	r.RegisterRuntimeCommand("devices", func(_ context.Context, rt *SessionRuntime, _ []string) (string, error) {
		return rt.connectors.Devices(), nil
	})
}

func (r *SessionRuntime) RegisterRuntimeCommand(name string, handler RuntimeCommandHandler) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || handler == nil {
		return
	}
	if r.runtimeCommands == nil {
		r.runtimeCommands = make(map[string]RuntimeCommandHandler)
	}
	r.runtimeCommands[name] = handler
}

func fillDefaultPorts(ports Ports) Ports {
	if ports.Runner == nil {
		ports.Runner = noopAgentRunner{}
	}
	if ports.Commands == nil {
		ports.Commands = noopCommandExecutor{}
	}
	if ports.Skills == nil {
		ports.Skills = noopSkillHintProvider{}
	}
	if ports.Catalog == nil {
		ports.Catalog = noopCommandCatalog{}
	}
	if ports.Control == nil {
		ports.Control = noopRunController{}
	}
	if ports.Stats == nil {
		ports.Stats = noopStatsProvider{}
	}
	if ports.Model == nil {
		ports.Model = noopModelInfoProvider{}
	}
	if ports.Messages == nil {
		ports.Messages = noopMessageProvider{}
	}
	return ports
}

type noopAgentRunner struct{}

func (noopAgentRunner) Run(string, RunCallbacks) (string, error) {
	return "", fmt.Errorf("runtime: no agent runner configured")
}

func (noopAgentRunner) ConfigureRuntime(ControlCallbacks) {}

type noopCommandExecutor struct{}

func (noopCommandExecutor) ExecuteCommand(string) (string, CmdResult) {
	return "", CmdNone
}

type noopSkillHintProvider struct{}

func (noopSkillHintProvider) SkillHint() (string, bool) { return "", false }

type noopCommandCatalog struct{}

func (noopCommandCatalog) CommandNames() []string { return nil }

type noopRunController struct{}

func (noopRunController) Steer(string) {}
func (noopRunController) Abort()       {}
func (noopRunController) Close()       {}

type noopStatsProvider struct{}

func (noopStatsProvider) Stats() BotStats { return BotStats{} }

type noopModelInfoProvider struct{}

func (noopModelInfoProvider) ProviderModel() (provider, model string) { return "", "" }

type noopMessageProvider struct{}

func (noopMessageProvider) SessionMessages() []DisplayMessage { return nil }

func (r *SessionRuntime) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.Subscribe(ctx, filter)
}

func (r *SessionRuntime) SubscribeReplay(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.SubscribeReplay(ctx, filter)
}

func (r *SessionRuntime) Publish(ev Event) {
	r.events.Publish(ev)
}

func (r *SessionRuntime) EnableEventRecording(baseDir string) error {
	if r.recorder != nil {
		return nil
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = defaultEventRecorderBaseDir()
	}
	if events, err := LoadRecordedEvents(baseDir); err == nil && len(events) > 0 {
		for _, ev := range events {
			r.runs.Record(ev)
		}
		r.events.ImportHistory(events)
		r.advanceRunSequence(events)
	}
	recorder, err := NewEventRecorder(baseDir)
	if err != nil {
		return err
	}
	r.recorder = recorder
	r.events.AddObserver(recorder.Record)
	return nil
}

func (r *SessionRuntime) advanceRunSequence(events []Event) {
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

func (r *SessionRuntime) EventRecordingSessionID() string {
	if r.recorder == nil {
		return ""
	}
	return r.recorder.SessionID()
}

func (r *SessionRuntime) CurrentRunID() RunID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentRun
}

func (r *SessionRuntime) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *SessionRuntime) PendingApprovals() []ApprovalView {
	return r.approvals.Pending()
}

func (r *SessionRuntime) PendingQuestions() []QuestionView {
	return r.questions.Pending()
}

func (r *SessionRuntime) CurrentRunView() (RunView, bool) {
	return r.runs.CurrentRunView()
}

func (r *SessionRuntime) RunView(runID RunID) (RunView, bool) {
	return r.runs.RunView(runID)
}

func (r *SessionRuntime) ListRunViews(limit int) []RunView {
	return r.runs.ListRunViews(limit)
}

func (r *SessionRuntime) ArtifactView(runID RunID) (ArtifactView, bool) {
	return r.runs.ArtifactView(runID)
}

func (r *SessionRuntime) ConnectView() ConnectView {
	return r.connectors.View()
}

func (r *SessionRuntime) EventHistory(filter EventFilter) []Event {
	return r.events.History(filter)
}

func (r *SessionRuntime) setStatus(status RunStatus) {
	r.mu.Lock()
	r.status = status
	r.mu.Unlock()
}

func (r *SessionRuntime) beginAbort(runID RunID) (RunID, bool, error) {
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

func (r *SessionRuntime) isRunAborted(runID RunID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.aborted[runID]
	return ok
}

func (r *SessionRuntime) shouldResumeRun(runID RunID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentRun != runID {
		return false
	}
	_, aborted := r.aborted[runID]
	return !aborted
}
