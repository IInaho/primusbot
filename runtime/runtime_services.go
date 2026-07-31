package runtime

import "fmt"

// MetricsProvider exposes operational measurements independently of runtime
// lifecycle status.
type MetricsProvider interface {
	Metrics() MetricsSnapshot
}

// ModelService is a runner-side optional read capability. Interaction surfaces
// query the current model through Manager.
type ModelService interface {
	CurrentModel() ModelSelection
}

// ModelMutator is a runner-side optional write capability. Manager remains the
// public mutation boundary and enforces lifecycle checks.
type ModelMutator interface {
	SwitchModel(name string) (ModelSelection, error)
}

// ContextService is a runner-side optional structured-context read capability.
type ContextService interface {
	ContextSnapshot() ContextSnapshot
	MemoryView(scope MemoryScope) MemoryView
}

// ExtensionService is a runner-side optional extension-state read capability.
type ExtensionService interface {
	SkillManagementView() SkillManagementView
}

// ExtensionMutator is a runner-side optional extension write capability.
type ExtensionMutator interface {
	SelectSkill(name string) error
	ClearSelectedSkill()
	RefreshSkillManagement() SkillManagementView
	SetPluginEnabled(name string, enabled bool) (SkillManagementView, error)
}

// ConfigurationService is a runner-side optional configuration read
// capability.
type ConfigurationService interface {
	ConfigView() ConfigView
}

// ConfigurationMutator is a runner-side optional configuration write
// capability.
type ConfigurationMutator interface {
	ApplyConfig(config ConfigView) (ConfigView, error)
}

// SessionService is a runner-side optional persisted-conversation read
// capability.
type SessionService interface {
	CurrentSessionID() string
	ListSessions() []SessionMeta
	SessionMessages() []DisplayMessage
}

// SessionMutator is a runner-side optional persisted-conversation write
// capability.
type SessionMutator interface {
	ResumeSession(id string) error
	NewSession() (SessionMeta, error)
	DeleteSession(id string) error
}

func (r *Manager) mutation(op string, supported bool, fn func() error) error {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return protocolError(ErrorClosed, op, "closed")
	}
	if !supported {
		r.mu.Unlock()
		return protocolError(ErrorUnsupported, op, "capability unavailable")
	}
	if r.status != RunIdle {
		r.mu.Unlock()
		return protocolError(ErrorBusy, op, fmt.Sprintf("run %s is %s", r.currentRun, r.status))
	}
	r.mutating = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.mutating = false
		r.mu.Unlock()
	}()
	return fn()
}

func (r *Manager) SwitchModel(name string) (selection ModelSelection, err error) {
	err = r.mutation("switch_model", r.modelMutator != nil, func() error {
		selection, err = r.modelMutator.SwitchModel(name)
		return err
	})
	return selection, err
}

func (r *Manager) SelectSkill(name string) error {
	return r.mutation("select_skill", r.extensionMutator != nil, func() error {
		return r.extensionMutator.SelectSkill(name)
	})
}

func (r *Manager) ClearSelectedSkill() error {
	return r.mutation("clear_selected_skill", r.extensionMutator != nil, func() error {
		r.extensionMutator.ClearSelectedSkill()
		return nil
	})
}

func (r *Manager) RefreshSkillManagement() (view SkillManagementView, err error) {
	err = r.mutation("refresh_extensions", r.extensionMutator != nil, func() error {
		view = r.extensionMutator.RefreshSkillManagement()
		return nil
	})
	return view, err
}

func (r *Manager) SetPluginEnabled(name string, enabled bool) (view SkillManagementView, err error) {
	err = r.mutation("set_plugin_enabled", r.extensionMutator != nil, func() error {
		view, err = r.extensionMutator.SetPluginEnabled(name, enabled)
		return err
	})
	return view, err
}

func (r *Manager) ApplyConfig(config ConfigView) (view ConfigView, err error) {
	err = r.mutation("apply_config", r.configMutator != nil, func() error {
		view, err = r.configMutator.ApplyConfig(config)
		return err
	})
	return view, err
}

func (r *Manager) ResumeSession(id string) error {
	err := r.mutation("resume_session", r.sessionMutator != nil, func() error {
		return r.sessionMutator.ResumeSession(id)
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return err
}

func (r *Manager) NewSession() (session SessionMeta, err error) {
	err = r.mutation("new_session", r.sessionMutator != nil, func() error {
		session, err = r.sessionMutator.NewSession()
		return err
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return session, err
}

func (r *Manager) DeleteSession(id string) error {
	err := r.mutation("delete_session", r.sessionMutator != nil, func() error {
		return r.sessionMutator.DeleteSession(id)
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return err
}

func (r *Manager) publishSessionChanged() {
	sessionID := ""
	if r.sessions != nil {
		sessionID = r.sessions.CurrentSessionID()
	}
	r.events.Publish(Event{
		Type: EventSessionChanged, Source: SourceRef{Kind: "runtime"},
		Payload: SessionPayload{ID: sessionID},
	})
}
