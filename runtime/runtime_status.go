package runtime

import (
	"sort"
	"strings"
	"time"
)

const metricsPublishInterval = 100 * time.Millisecond

// CapabilityManifest lets an interaction surface discover optional features
// without probing methods or depending on a concrete bot.
type CapabilityManifest struct {
	Protocol      string `json:"protocol"`
	Steering      bool   `json:"steering"`
	Commands      bool   `json:"commands"`
	Metrics       bool   `json:"metrics"`
	Models        bool   `json:"models"`
	Context       bool   `json:"context"`
	Extensions    bool   `json:"extensions"`
	Configuration bool   `json:"configuration"`
	Sessions      bool   `json:"sessions"`
	Connectors    bool   `json:"connectors"`
}

type RuntimeState string

const (
	RuntimeReady  RuntimeState = "ready"
	RuntimeBusy   RuntimeState = "busy"
	RuntimeClosed RuntimeState = "closed"
)

// RuntimeStatus is the small, stable health and lifecycle snapshot.
type RuntimeStatus struct {
	State        RuntimeState       `json:"state"`
	ActiveRun    RunID              `json:"active_run,omitempty"`
	RunStatus    RunStatus          `json:"run_status"`
	Capabilities CapabilityManifest `json:"capabilities"`
}

func (r *Manager) Capabilities() CapabilityManifest {
	r.mu.Lock()
	hasCommands := r.commander != nil || len(r.runtimeCommands) > 0
	r.mu.Unlock()
	return CapabilityManifest{
		Protocol: ProtocolVersion, Steering: r.steerer != nil,
		Commands: hasCommands,
		Metrics:  r.metrics != nil, Models: r.models != nil,
		Context: r.context != nil, Extensions: r.extensions != nil,
		Configuration: r.configuration != nil, Sessions: r.sessions != nil,
		Connectors: len(r.connectors.View().Connectors) > 0,
	}
}

func (r *Manager) Status() RuntimeStatus {
	r.mu.Lock()
	state := RuntimeReady
	if r.closed {
		state = RuntimeClosed
	} else if r.status != RunIdle || r.mutating {
		state = RuntimeBusy
	}
	activeRun := r.currentRun
	if r.status == RunIdle {
		activeRun = ""
	}
	runStatus := r.status
	r.mu.Unlock()
	return RuntimeStatus{
		State: state, ActiveRun: activeRun, RunStatus: runStatus,
		Capabilities: r.Capabilities(),
	}
}

// Optional read models are queried directly through Manager. Call
// Capabilities to discover availability; unavailable or closed capabilities
// return their zero value.
func (r *Manager) CurrentModel() ModelSelection {
	r.mu.Lock()
	service, closed := r.models, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return ModelSelection{}
	}
	return service.CurrentModel()
}

func (r *Manager) ContextSnapshot() ContextSnapshot {
	r.mu.Lock()
	service, closed := r.context, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return ContextSnapshot{}
	}
	return service.ContextSnapshot()
}

func (r *Manager) MemoryView(scope MemoryScope) MemoryView {
	r.mu.Lock()
	service, closed := r.context, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return MemoryView{}
	}
	return service.MemoryView(scope)
}

func (r *Manager) SkillManagementView() SkillManagementView {
	r.mu.Lock()
	service, closed := r.extensions, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return SkillManagementView{}
	}
	return service.SkillManagementView()
}

func (r *Manager) ConfigView() ConfigView {
	r.mu.Lock()
	service, closed := r.configuration, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return ConfigView{}
	}
	return service.ConfigView()
}

func (r *Manager) CurrentSessionID() string {
	r.mu.Lock()
	service, closed := r.sessions, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return ""
	}
	return service.CurrentSessionID()
}

func (r *Manager) ListSessions() []SessionMeta {
	r.mu.Lock()
	service, closed := r.sessions, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return nil
	}
	return service.ListSessions()
}

func (r *Manager) SessionMessages() []DisplayMessage {
	r.mu.Lock()
	service, closed := r.sessions, r.closed
	r.mu.Unlock()
	if closed || service == nil {
		return nil
	}
	return service.SessionMessages()
}

func (r *Manager) publishMetrics(runID RunID) {
	if r.metrics == nil {
		return
	}
	metrics := r.metrics.Metrics()
	r.events.Publish(Event{
		RunID: runID, Type: EventMetricsUpdated, Source: SourceRef{Kind: "bot"},
		Payload: metrics,
	})
}

func (r *Manager) startMetricsUpdates(runID RunID, lease *runLease) func() {
	if r.metrics == nil {
		return func() {}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(metricsPublishInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !lease.guard(func() { r.publishMetrics(runID) }) {
					return
				}
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func (r *Manager) recordMetrics(event Event) {
	if event.Type != EventMetricsUpdated {
		return
	}
	metrics, ok := event.Payload.(MetricsSnapshot)
	if !ok {
		return
	}
	r.mu.Lock()
	r.latestMetrics = metrics
	r.mu.Unlock()
}

func (r *Manager) CurrentRun() (RunSnapshot, bool) {
	return r.runs.Current()
}

func (r *Manager) LookupRun(runID RunID) (RunSnapshot, bool) {
	return r.runs.Lookup(runID)
}

func (r *Manager) Runs(limit int) []RunSnapshot {
	return r.runs.List(limit)
}

func (r *Manager) Metrics() MetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latestMetrics
}

func (r *Manager) CommandCatalog() []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	if r.commander != nil {
		for _, name := range r.commander.CommandNames() {
			display := commandDisplayName(name)
			if display == "" || seen[display] {
				continue
			}
			seen[display] = true
			names = append(names, display)
		}
	}
	r.mu.Lock()
	runtimeNames := make([]string, 0, len(r.runtimeCommands))
	for name := range r.runtimeCommands {
		runtimeNames = append(runtimeNames, name)
	}
	r.mu.Unlock()
	for _, name := range runtimeNames {
		display := commandDisplayName(name)
		if seen[display] {
			continue
		}
		seen[display] = true
		names = append(names, display)
	}
	sort.Strings(names)
	return names
}

func commandDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "$") {
		return name
	}
	return "/" + name
}
