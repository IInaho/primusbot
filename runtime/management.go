package runtime

import "fmt"

// Optional management capabilities are discovered on the same Backend
// instance. Unsupported operations return an explicit error or zero view.

func unsupportedManagementCapability(name string) error {
	return fmt.Errorf("runtime: bot does not support %s management", name)
}

func (r *Manager) SwitchModel(name string) (string, string, error) {
	manager, ok := r.backend.(modelManager)
	if !ok {
		return "", "", unsupportedManagementCapability("model")
	}
	return manager.SwitchModel(name)
}

func (r *Manager) ContextStatus() string {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return ""
	}
	return manager.ContextStatus()
}

func (r *Manager) ContextReport() string {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return unsupportedManagementCapability("context").Error()
	}
	return manager.ContextReport()
}

func (r *Manager) ContextSnapshot() ContextSnapshot {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return ContextSnapshot{}
	}
	return manager.ContextSnapshot()
}

func (r *Manager) MemoryView(scope MemoryScope) MemoryView {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return MemoryView{}
	}
	return manager.MemoryView(scope)
}

func (r *Manager) CWD() string {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return ""
	}
	return manager.CWD()
}

func (r *Manager) ClearContext() {
	manager, ok := r.backend.(contextManager)
	if !ok {
		return
	}
	manager.ClearContext()
}

func (r *Manager) SelectSkill(name string) error {
	manager, ok := r.backend.(skillManager)
	if !ok {
		return unsupportedManagementCapability("skill")
	}
	return manager.SelectSkill(name)
}

func (r *Manager) ClearSelectedSkill() {
	manager, ok := r.backend.(skillManager)
	if !ok {
		return
	}
	manager.ClearSelectedSkill()
}

func (r *Manager) SkillManagementView() SkillManagementView {
	manager, ok := r.backend.(skillManager)
	if !ok {
		return SkillManagementView{}
	}
	return manager.SkillManagementView()
}

func (r *Manager) RefreshSkillManagement() SkillManagementView {
	manager, ok := r.backend.(skillManager)
	if !ok {
		return SkillManagementView{}
	}
	return manager.RefreshSkillManagement()
}

func (r *Manager) SetPluginEnabled(name string, enabled bool) (SkillManagementView, error) {
	manager, ok := r.backend.(skillManager)
	if !ok {
		return SkillManagementView{}, unsupportedManagementCapability("skill")
	}
	return manager.SetPluginEnabled(name, enabled)
}

func (r *Manager) ConfigView() ConfigView {
	manager, ok := r.backend.(configManager)
	if !ok {
		return ConfigView{}
	}
	return manager.ConfigView()
}

func (r *Manager) ApplyConfig(cfg ConfigView) (ConfigView, error) {
	manager, ok := r.backend.(configManager)
	if !ok {
		return ConfigView{}, unsupportedManagementCapability("config")
	}
	return manager.ApplyConfig(cfg)
}

func (r *Manager) CurrentSessionID() string {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return ""
	}
	return manager.CurrentSessionID()
}

func (r *Manager) SetSession(id string) error {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return unsupportedManagementCapability("session")
	}
	return manager.SetSession(id)
}

func (r *Manager) ResumeSession(id string) error {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return unsupportedManagementCapability("session")
	}
	return manager.ResumeSession(id)
}

func (r *Manager) ListSessions() []SessionMeta {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return nil
	}
	return manager.ListSessions()
}

func (r *Manager) NewSession() (SessionMeta, error) {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return SessionMeta{}, unsupportedManagementCapability("session")
	}
	return manager.NewSession()
}

func (r *Manager) DeleteSession(id string) error {
	manager, ok := r.backend.(sessionManager)
	if !ok {
		return unsupportedManagementCapability("session")
	}
	return manager.DeleteSession(id)
}
