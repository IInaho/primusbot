package runtime

import (
	"fmt"
)

func unsupportedManagementCapability(name string) error {
	return fmt.Errorf("runtime: bot does not support %s management", name)
}

func (r *SessionRuntime) modelManager() (CoreModelManager, error) {
	if r.modelManagement == nil {
		return nil, unsupportedManagementCapability("model")
	}
	return r.modelManagement, nil
}

func (r *SessionRuntime) contextManager() (CoreContextManager, error) {
	if r.contextManagement == nil {
		return nil, unsupportedManagementCapability("context")
	}
	return r.contextManagement, nil
}

func (r *SessionRuntime) skillManager() (CoreSkillManager, error) {
	if r.skillManagement == nil {
		return nil, unsupportedManagementCapability("skill")
	}
	return r.skillManagement, nil
}

func (r *SessionRuntime) configManager() (CoreConfigManager, error) {
	if r.configManagement == nil {
		return nil, unsupportedManagementCapability("config")
	}
	return r.configManagement, nil
}

func (r *SessionRuntime) sessionManager() (CoreSessionManager, error) {
	if r.sessionManagement == nil {
		return nil, unsupportedManagementCapability("session")
	}
	return r.sessionManagement, nil
}

func (r *SessionRuntime) SwitchModel(name string) (string, string, error) {
	manager, err := r.modelManager()
	if err != nil {
		return "", "", err
	}
	return manager.SwitchModel(name)
}

func (r *SessionRuntime) ContextStatus() string {
	manager, err := r.contextManager()
	if err != nil {
		return ""
	}
	return manager.ContextStatus()
}

func (r *SessionRuntime) ContextReport() string {
	manager, err := r.contextManager()
	if err != nil {
		return err.Error()
	}
	return manager.ContextReport()
}

func (r *SessionRuntime) ContextSnapshot() ContextSnapshot {
	manager, err := r.contextManager()
	if err != nil {
		return ContextSnapshot{}
	}
	return manager.ContextSnapshot()
}

func (r *SessionRuntime) MemoryView(scope MemoryScope) MemoryView {
	manager, err := r.contextManager()
	if err != nil {
		return MemoryView{}
	}
	return manager.MemoryView(scope)
}

func (r *SessionRuntime) SelectSkill(name string) error {
	manager, err := r.skillManager()
	if err != nil {
		return err
	}
	return manager.SelectSkill(name)
}

func (r *SessionRuntime) ClearSelectedSkill() {
	manager, err := r.skillManager()
	if err != nil {
		return
	}
	manager.ClearSelectedSkill()
}

func (r *SessionRuntime) ConfigView() ConfigView {
	manager, err := r.configManager()
	if err != nil {
		return ConfigView{}
	}
	return manager.ConfigView()
}

func (r *SessionRuntime) ApplyConfig(cfg ConfigView) (ConfigView, error) {
	manager, err := r.configManager()
	if err != nil {
		return ConfigView{}, err
	}
	return manager.ApplyConfig(cfg)
}

func (r *SessionRuntime) SkillManagementView() SkillManagementView {
	manager, err := r.skillManager()
	if err != nil {
		return SkillManagementView{}
	}
	return manager.SkillManagementView()
}

func (r *SessionRuntime) RefreshSkillManagement() SkillManagementView {
	manager, err := r.skillManager()
	if err != nil {
		return SkillManagementView{}
	}
	return manager.RefreshSkillManagement()
}

func (r *SessionRuntime) SetPluginEnabled(name string, enabled bool) (SkillManagementView, error) {
	manager, err := r.skillManager()
	if err != nil {
		return SkillManagementView{}, err
	}
	return manager.SetPluginEnabled(name, enabled)
}

func (r *SessionRuntime) CWD() string {
	manager, err := r.contextManager()
	if err != nil {
		return ""
	}
	return manager.CWD()
}

func (r *SessionRuntime) ClearContext() {
	manager, err := r.contextManager()
	if err != nil {
		return
	}
	manager.ClearContext()
}

func (r *SessionRuntime) CurrentSessionID() string {
	manager, err := r.sessionManager()
	if err != nil {
		return ""
	}
	return manager.CurrentSessionID()
}

func (r *SessionRuntime) SetSession(id string) error {
	manager, err := r.sessionManager()
	if err != nil {
		return err
	}
	return manager.SetSession(id)
}

func (r *SessionRuntime) ResumeSession(id string) error {
	manager, err := r.sessionManager()
	if err != nil {
		return err
	}
	return manager.ResumeSession(id)
}

func (r *SessionRuntime) ListSessions() []SessionMeta {
	manager, err := r.sessionManager()
	if err != nil {
		return nil
	}
	return manager.ListSessions()
}

func (r *SessionRuntime) NewSession() (SessionMeta, error) {
	manager, err := r.sessionManager()
	if err != nil {
		return SessionMeta{}, err
	}
	return manager.NewSession()
}

func (r *SessionRuntime) DeleteSession(id string) error {
	manager, err := r.sessionManager()
	if err != nil {
		return err
	}
	return manager.DeleteSession(id)
}
