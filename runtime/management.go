package runtime

import (
	"fmt"

	"nekocode/runtime/view"
)

func (r *SessionRuntime) guiBot() (GUIBot, error) {
	gui, ok := r.bot.(GUIBot)
	if !ok {
		return nil, fmt.Errorf("runtime: bot does not implement GUI interface")
	}
	return gui, nil
}

func (r *SessionRuntime) SwitchModel(name string) (string, string, error) {
	gui, err := r.guiBot()
	if err != nil {
		return "", "", err
	}
	return gui.SwitchModel(name)
}

func (r *SessionRuntime) ContextStatus() string {
	gui, err := r.guiBot()
	if err != nil {
		return ""
	}
	return gui.ContextStatus()
}

func (r *SessionRuntime) ContextReport() string {
	gui, err := r.guiBot()
	if err != nil {
		return err.Error()
	}
	return gui.ContextReport()
}

func (r *SessionRuntime) ContextSnapshot() view.ContextSnapshot {
	gui, err := r.guiBot()
	if err != nil {
		return view.ContextSnapshot{}
	}
	return gui.ContextSnapshot()
}

func (r *SessionRuntime) SelectSkill(name string) error {
	gui, err := r.guiBot()
	if err != nil {
		return err
	}
	return gui.SelectSkill(name)
}

func (r *SessionRuntime) ClearSelectedSkill() {
	gui, err := r.guiBot()
	if err != nil {
		return
	}
	gui.ClearSelectedSkill()
}

func (r *SessionRuntime) ConfigView() view.ConfigView {
	gui, err := r.guiBot()
	if err != nil {
		return view.ConfigView{}
	}
	return gui.ConfigView()
}

func (r *SessionRuntime) ApplyConfig(cfg view.ConfigView) (view.ConfigView, error) {
	gui, err := r.guiBot()
	if err != nil {
		return view.ConfigView{}, err
	}
	return gui.ApplyConfig(cfg)
}

func (r *SessionRuntime) SkillManagementView() view.SkillManagementView {
	gui, err := r.guiBot()
	if err != nil {
		return view.SkillManagementView{}
	}
	return gui.SkillManagementView()
}

func (r *SessionRuntime) RefreshSkillManagement() view.SkillManagementView {
	gui, err := r.guiBot()
	if err != nil {
		return view.SkillManagementView{}
	}
	return gui.RefreshSkillManagement()
}

func (r *SessionRuntime) SetPluginEnabled(name string, enabled bool) (view.SkillManagementView, error) {
	gui, err := r.guiBot()
	if err != nil {
		return view.SkillManagementView{}, err
	}
	return gui.SetPluginEnabled(name, enabled)
}

func (r *SessionRuntime) CWD() string {
	gui, err := r.guiBot()
	if err != nil {
		return ""
	}
	return gui.CWD()
}

func (r *SessionRuntime) ClearContext() {
	gui, err := r.guiBot()
	if err != nil {
		return
	}
	gui.ClearContext()
}

func (r *SessionRuntime) CurrentSessionID() string {
	gui, err := r.guiBot()
	if err != nil {
		return ""
	}
	return gui.CurrentSessionID()
}

func (r *SessionRuntime) SetSession(id string) error {
	gui, err := r.guiBot()
	if err != nil {
		return err
	}
	return gui.SetSession(id)
}

func (r *SessionRuntime) ResumeSession(id string) error {
	gui, err := r.guiBot()
	if err != nil {
		return err
	}
	return gui.ResumeSession(id)
}

func (r *SessionRuntime) ListSessions() []view.SessionMeta {
	gui, err := r.guiBot()
	if err != nil {
		return nil
	}
	return gui.ListSessions()
}

func (r *SessionRuntime) NewSession() (view.SessionMeta, error) {
	gui, err := r.guiBot()
	if err != nil {
		return view.SessionMeta{}, err
	}
	return gui.NewSession()
}

func (r *SessionRuntime) DeleteSession(id string) error {
	gui, err := r.guiBot()
	if err != nil {
		return err
	}
	return gui.DeleteSession(id)
}
