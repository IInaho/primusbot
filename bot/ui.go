package bot

import (
	"nekocode/bot/extension"
	"nekocode/bot/view"
)

type UI interface {
	Run(input string, callbacks view.RunCallbacks) (string, error)
	ExecuteCommand(input string) (string, view.CmdResult)
	SkillHint() (string, bool)
	Stats() view.BotStats
	CommandNames() []string
	Configure(confirmFn view.ConfirmFunc, phaseFn view.PhaseFunc, todoFn view.TodoFunc, notifyFn func(string), confirmCh chan view.ConfirmRequest, questionFn view.QuestionFunc)
	Steer(msg string)
	Abort()
	Close()
	ProviderModel() (provider, model string)
	SwitchModel(name string) (model, provider string, err error)
	ContextStatus() string
	ContextReport() string
	ContextSnapshot() view.ContextSnapshot
	SelectSkill(name string) error
	ClearSelectedSkill()
	SessionMessages() []view.DisplayMessage
}

type GUI interface {
	UI
	ConfigView() view.ConfigView
	ApplyConfig(view view.ConfigView) (view.ConfigView, error)
	SkillManagementView() extension.SkillManagementView
	RefreshSkillManagement() extension.SkillManagementView
	SetPluginEnabled(name string, enabled bool) (extension.SkillManagementView, error)
	CWD() string
	ClearContext()
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []view.SessionMeta
	NewSession() (view.SessionMeta, error)
	DeleteSession(id string) error
}
