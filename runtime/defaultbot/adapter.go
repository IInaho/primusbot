package defaultbot

import (
	commonview "nekocode/common/view"
	controlruntime "nekocode/runtime"
)

type botFacade interface {
	Run(input string, callbacks commonview.RunCallbacks) (string, error)
	ConfigureRuntime(callbacks commonview.ControlCallbacks)
	ExecuteCommand(input string) (string, commonview.CmdResult)
	SkillHint() (string, bool)
	CommandNames() []string
	Steer(msg string)
	Abort()
	Close()
	Stats() commonview.BotStats
	ProviderModel() (provider, model string)
	SessionMessages() []commonview.DisplayMessage

	SwitchModel(name string) (model, provider string, err error)
	ContextStatus() string
	ContextReport() string
	ContextSnapshot() commonview.ContextSnapshot
	MemoryView(scope commonview.MemoryScope) commonview.MemoryView
	CWD() string
	ClearContext()
	SelectSkill(name string) error
	ClearSelectedSkill()
	SkillManagementView() commonview.SkillManagementView
	RefreshSkillManagement() commonview.SkillManagementView
	SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error)
	ConfigView() commonview.ConfigView
	ApplyConfig(cfg commonview.ConfigView) (commonview.ConfigView, error)
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []commonview.SessionMeta
	NewSession() (commonview.SessionMeta, error)
	DeleteSession(id string) error
}

type adapter struct {
	bot           botFacade
	confirmChStop chan struct{}
}

func coreOptions(bot botFacade) controlruntime.CoreSessionRuntimeOptions {
	a := &adapter{bot: bot}
	return controlruntime.CoreSessionRuntimeOptions{
		Runner:            a,
		Commands:          a,
		Skills:            a,
		Catalog:           a,
		Control:           a,
		Stats:             a,
		Model:             a,
		Messages:          a,
		ModelManagement:   a,
		ContextManagement: a,
		SkillManagement:   a,
		ConfigManagement:  a,
		SessionManagement: a,
	}
}

func (a adapter) Run(input string, callbacks controlruntime.RunCallbacks) (string, error) {
	return a.bot.Run(input, callbacks)
}

func (a *adapter) ConfigureRuntime(callbacks controlruntime.ControlCallbacks) {
	if a.confirmChStop != nil {
		close(a.confirmChStop)
	}
	a.confirmChStop = make(chan struct{})
	viewCallbacks := commonview.ControlCallbacks(callbacks)
	if callbacks.ConfirmCh != nil {
		viewCallbacks.ConfirmCh = make(chan commonview.ConfirmRequest, 1)
		go a.bridgeConfirmCh(callbacks.ConfirmCh, viewCallbacks.ConfirmCh, a.confirmChStop)
	}
	a.bot.ConfigureRuntime(viewCallbacks)
}

func (a *adapter) bridgeConfirmCh(coreCh chan commonview.ConfirmRequest, viewCh chan commonview.ConfirmRequest, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case req, ok := <-viewCh:
			if !ok {
				return
			}
			if req.Response != nil {
				// Runtime handles approvals via the synchronous Confirm callback.
				// Non-nil Response on ConfirmCh is not expected; ignore to avoid
				// leaking goroutines waiting for a reply.
				continue
			}
			select {
			case <-stop:
				return
			case coreCh <- req:
			}
		}
	}
}

func (a adapter) ExecuteCommand(input string) (string, controlruntime.CmdResult) {
	return a.bot.ExecuteCommand(input)
}

func (a adapter) SkillHint() (string, bool) { return a.bot.SkillHint() }
func (a adapter) CommandNames() []string    { return a.bot.CommandNames() }
func (a adapter) Steer(msg string)          { a.bot.Steer(msg) }
func (a adapter) Abort()                    { a.bot.Abort() }

func (a *adapter) Close() {
	if a.confirmChStop != nil {
		close(a.confirmChStop)
	}
	a.bot.Close()
}

func (a adapter) Stats() controlruntime.BotStats {
	return a.bot.Stats()
}

func (a adapter) ProviderModel() (provider, model string) {
	return a.bot.ProviderModel()
}

func (a adapter) SessionMessages() []controlruntime.DisplayMessage {
	return a.bot.SessionMessages()
}

func (a adapter) SwitchModel(name string) (model, provider string, err error) {
	return a.bot.SwitchModel(name)
}

func (a adapter) ContextStatus() string { return a.bot.ContextStatus() }
func (a adapter) ContextReport() string { return a.bot.ContextReport() }

func (a adapter) ContextSnapshot() controlruntime.ContextSnapshot {
	return a.bot.ContextSnapshot()
}

func (a adapter) MemoryView(scope controlruntime.MemoryScope) controlruntime.MemoryView {
	return a.bot.MemoryView(scope)
}

func (a adapter) CWD() string   { return a.bot.CWD() }
func (a adapter) ClearContext() { a.bot.ClearContext() }

func (a adapter) SelectSkill(name string) error { return a.bot.SelectSkill(name) }
func (a adapter) ClearSelectedSkill()           { a.bot.ClearSelectedSkill() }

func (a adapter) SkillManagementView() controlruntime.SkillManagementView {
	return a.bot.SkillManagementView()
}

func (a adapter) RefreshSkillManagement() controlruntime.SkillManagementView {
	return a.bot.RefreshSkillManagement()
}

func (a adapter) SetPluginEnabled(name string, enabled bool) (controlruntime.SkillManagementView, error) {
	return a.bot.SetPluginEnabled(name, enabled)
}

func (a adapter) ConfigView() controlruntime.ConfigView {
	return a.bot.ConfigView()
}

func (a adapter) ApplyConfig(cfg controlruntime.ConfigView) (controlruntime.ConfigView, error) {
	return a.bot.ApplyConfig(cfg)
}

func (a adapter) CurrentSessionID() string      { return a.bot.CurrentSessionID() }
func (a adapter) SetSession(id string) error    { return a.bot.SetSession(id) }
func (a adapter) ResumeSession(id string) error { return a.bot.ResumeSession(id) }

func (a adapter) ListSessions() []controlruntime.SessionMeta {
	return a.bot.ListSessions()
}

func (a adapter) NewSession() (controlruntime.SessionMeta, error) {
	return a.bot.NewSession()
}

func (a adapter) DeleteSession(id string) error { return a.bot.DeleteSession(id) }
