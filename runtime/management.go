package runtime

import "fmt"

// management.go — ManagementRuntime 的实现：每个管理能力都是可选端口，
// 未注入时 error 方法返回"不支持"，其余方法返回零值。

func unsupportedManagementCapability(name string) error {
	return fmt.Errorf("runtime: bot does not support %s management", name)
}

func (r *SessionRuntime) SwitchModel(name string) (string, string, error) {
	if r.modelManagement == nil {
		return "", "", unsupportedManagementCapability("model")
	}
	return r.modelManagement.SwitchModel(name)
}

func (r *SessionRuntime) ContextStatus() string {
	if r.contextManagement == nil {
		return ""
	}
	return r.contextManagement.ContextStatus()
}

func (r *SessionRuntime) ContextReport() string {
	if r.contextManagement == nil {
		return unsupportedManagementCapability("context").Error()
	}
	return r.contextManagement.ContextReport()
}

func (r *SessionRuntime) ContextSnapshot() ContextSnapshot {
	if r.contextManagement == nil {
		return ContextSnapshot{}
	}
	return r.contextManagement.ContextSnapshot()
}

func (r *SessionRuntime) MemoryView(scope MemoryScope) MemoryView {
	if r.contextManagement == nil {
		return MemoryView{}
	}
	return r.contextManagement.MemoryView(scope)
}

func (r *SessionRuntime) CWD() string {
	if r.contextManagement == nil {
		return ""
	}
	return r.contextManagement.CWD()
}

func (r *SessionRuntime) ClearContext() {
	if r.contextManagement == nil {
		return
	}
	r.contextManagement.ClearContext()
}

func (r *SessionRuntime) SelectSkill(name string) error {
	if r.skillManagement == nil {
		return unsupportedManagementCapability("skill")
	}
	return r.skillManagement.SelectSkill(name)
}

func (r *SessionRuntime) ClearSelectedSkill() {
	if r.skillManagement == nil {
		return
	}
	r.skillManagement.ClearSelectedSkill()
}

func (r *SessionRuntime) SkillManagementView() SkillManagementView {
	if r.skillManagement == nil {
		return SkillManagementView{}
	}
	return r.skillManagement.SkillManagementView()
}

func (r *SessionRuntime) RefreshSkillManagement() SkillManagementView {
	if r.skillManagement == nil {
		return SkillManagementView{}
	}
	return r.skillManagement.RefreshSkillManagement()
}

func (r *SessionRuntime) SetPluginEnabled(name string, enabled bool) (SkillManagementView, error) {
	if r.skillManagement == nil {
		return SkillManagementView{}, unsupportedManagementCapability("skill")
	}
	return r.skillManagement.SetPluginEnabled(name, enabled)
}

func (r *SessionRuntime) ConfigView() ConfigView {
	if r.configManagement == nil {
		return ConfigView{}
	}
	return r.configManagement.ConfigView()
}

func (r *SessionRuntime) ApplyConfig(cfg ConfigView) (ConfigView, error) {
	if r.configManagement == nil {
		return ConfigView{}, unsupportedManagementCapability("config")
	}
	return r.configManagement.ApplyConfig(cfg)
}

func (r *SessionRuntime) CurrentSessionID() string {
	if r.sessionManagement == nil {
		return ""
	}
	return r.sessionManagement.CurrentSessionID()
}

func (r *SessionRuntime) SetSession(id string) error {
	if r.sessionManagement == nil {
		return unsupportedManagementCapability("session")
	}
	return r.sessionManagement.SetSession(id)
}

func (r *SessionRuntime) ResumeSession(id string) error {
	if r.sessionManagement == nil {
		return unsupportedManagementCapability("session")
	}
	return r.sessionManagement.ResumeSession(id)
}

func (r *SessionRuntime) ListSessions() []SessionMeta {
	if r.sessionManagement == nil {
		return nil
	}
	return r.sessionManagement.ListSessions()
}

func (r *SessionRuntime) NewSession() (SessionMeta, error) {
	if r.sessionManagement == nil {
		return SessionMeta{}, unsupportedManagementCapability("session")
	}
	return r.sessionManagement.NewSession()
}

func (r *SessionRuntime) DeleteSession(id string) error {
	if r.sessionManagement == nil {
		return unsupportedManagementCapability("session")
	}
	return r.sessionManagement.DeleteSession(id)
}
