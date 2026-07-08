package bot

import (
	"nekocode/bot/extension"
	"nekocode/common"
	"nekocode/common/ui"
)

type UI interface {
	Run(input string, callbacks common.RunCallbacks) (string, error)
	ExecuteCommand(input string) (string, common.CmdResult)
	SkillHint() (string, bool)
	Stats() common.BotStats
	CommandNames() []string
	Configure(confirmFn common.ConfirmFunc, phaseFn common.PhaseFunc, todoFn common.TodoFunc, notifyFn func(string), confirmCh chan common.ConfirmRequest, questionFn common.QuestionFunc)
	Steer(msg string)
	Abort()
	Close()
	ProviderModel() (provider, model string)
	SwitchModel(name string) (model, provider string, err error)
	ContextStatus() string
	ContextReport() string
	ContextSnapshot() common.ContextSnapshot
	SelectSkill(name string) error
	ClearSelectedSkill()
	SessionMessages() []common.DisplayMessage
}

type GUI interface {
	UI
	ConfigView() ui.ConfigView
	ApplyConfig(view ui.ConfigView) (ui.ConfigView, error)
	SkillManagementView() extension.SkillManagementView
	RefreshSkillManagement() extension.SkillManagementView
	SetPluginEnabled(name string, enabled bool) (extension.SkillManagementView, error)
	CWD() string
	ClearContext()
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []ui.SessionMeta
	NewSession() (ui.SessionMeta, error)
	DeleteSession(id string) error
}
