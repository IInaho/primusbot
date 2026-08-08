package runtime

import (
	"context"
	"fmt"
)

// Services explicitly describes the optional application functions attached
// to a Runner. The composition root supplies this value once; Manager and
// transports do not discover capabilities through type assertions.
type Services struct {
	ExecuteCommand         func(ctx context.Context, input string, host RunHost) (CommandResult, error)
	ExecuteLocalCommand    func(context.Context, string) (string, LocalCommandResult)
	CommandMenu            func(context.Context, string) (CommandMenu, bool)
	Steer                  func(ctx context.Context, message string) error
	Metrics                func() MetricsSnapshot
	CurrentModel           func() ModelSelection
	PermissionMode         func() string
	SwitchModel            func(string) (ModelSelection, error)
	ContextSnapshot        func() ContextSnapshot
	WorkspaceChanges       func() WorkspaceChanges
	MemoryView             func(MemoryScope) MemoryView
	SkillManagementView    func() SkillManagementView
	SelectSkill            func(string) error
	ClearSelectedSkill     func()
	RefreshSkillManagement func() SkillManagementView
	SetPluginEnabled       func(string, bool) (SkillManagementView, error)
	ConfigView             func() ConfigView
	ResolveModelProfile    func(ModelSpec) ModelProfile
	ApplyConfig            func(ConfigView) (ConfigView, error)
	CurrentSessionID       func() string
	ListSessions           func() []SessionMeta
	SessionMessages        func() []DisplayMessage
	ResumeSession          func(string) error
	NewSession             func() (SessionMeta, error)
	DeleteSession          func(string) error
	Close                  func() error
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
	err = r.mutation("switch_model", r.services.SwitchModel != nil, func() error {
		selection, err = r.services.SwitchModel(name)
		return err
	})
	return selection, err
}

func (r *Manager) SelectSkill(name string) error {
	return r.mutation("select_skill", r.services.SelectSkill != nil, func() error {
		return r.services.SelectSkill(name)
	})
}

func (r *Manager) ClearSelectedSkill() error {
	return r.mutation("clear_selected_skill", r.services.ClearSelectedSkill != nil, func() error {
		r.services.ClearSelectedSkill()
		return nil
	})
}

func (r *Manager) RefreshSkillManagement() (view SkillManagementView, err error) {
	err = r.mutation("refresh_extensions", r.services.RefreshSkillManagement != nil, func() error {
		view = r.services.RefreshSkillManagement()
		return nil
	})
	return view, err
}

func (r *Manager) SetPluginEnabled(name string, enabled bool) (view SkillManagementView, err error) {
	err = r.mutation("set_plugin_enabled", r.services.SetPluginEnabled != nil, func() error {
		view, err = r.services.SetPluginEnabled(name, enabled)
		return err
	})
	return view, err
}

func (r *Manager) ApplyConfig(config ConfigView) (view ConfigView, err error) {
	err = r.mutation("apply_config", r.services.ApplyConfig != nil, func() error {
		view, err = r.services.ApplyConfig(config)
		return err
	})
	return view, err
}

func (r *Manager) ResumeSession(id string) error {
	err := r.mutation("resume_session", r.services.ResumeSession != nil, func() error {
		return r.services.ResumeSession(id)
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return err
}

func (r *Manager) NewSession() (session SessionMeta, err error) {
	err = r.mutation("new_session", r.services.NewSession != nil, func() error {
		session, err = r.services.NewSession()
		return err
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return session, err
}

func (r *Manager) DeleteSession(id string) error {
	err := r.mutation("delete_session", r.services.DeleteSession != nil, func() error {
		return r.services.DeleteSession(id)
	})
	if err == nil {
		r.publishSessionChanged()
	}
	return err
}

func (r *Manager) publishSessionChanged() {
	sessionID := ""
	if r.services.CurrentSessionID != nil {
		sessionID = r.services.CurrentSessionID()
	}
	r.events.Publish(Event{
		Type: EventSessionChanged, Source: SourceRef{Kind: "runtime"},
		Payload: SessionPayload{ID: sessionID},
	})
}
