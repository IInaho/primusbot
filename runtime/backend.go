package runtime

import commonview "nekocode/common/view"

type CmdResult = commonview.CmdResult

const (
	CmdNone           = commonview.CmdNone
	CmdHandled        = commonview.CmdHandled
	CmdConfirming     = commonview.CmdConfirming
	CmdSessionResumed = commonview.CmdSessionResumed
)

type StepAction = commonview.StepAction
type StepEvent = commonview.StepEvent

const (
	StepActionChat           = commonview.StepActionChat
	StepActionThink          = commonview.StepActionThink
	StepActionToolStart      = commonview.StepActionToolStart
	StepActionToolBlocked    = commonview.StepActionToolBlocked
	StepActionToolPreview    = commonview.StepActionToolPreview
	StepActionExecuteTool    = commonview.StepActionExecuteTool
	StepActionSubToolStart   = commonview.StepActionSubToolStart
	StepActionSubExecuteTool = commonview.StepActionSubExecuteTool
	StepActionSubAgentStart  = commonview.StepActionSubAgentStart
	StepActionSubAgentEnd    = commonview.StepActionSubAgentEnd
)

type RunCallbacks = commonview.RunCallbacks
type ControlCallbacks = commonview.ControlCallbacks

// Backend is the bot capability required by Manager. Applications construct
// one backend and pass that single instance to New; runtime owns all
// interaction orchestration above it.
type Backend interface {
	Run(input string, callbacks RunCallbacks) (string, error)
	ConfigureRuntime(callbacks ControlCallbacks)
	ExecuteCommand(input string) (string, CmdResult)
	SkillHint() (string, bool)
	CommandNames() []string
	Steer(msg string)
	Abort()
	Close()
	Stats() commonview.BotStats
	ProviderModel() (provider, model string)
	SessionMessages() []commonview.DisplayMessage
}

type modelManager interface {
	SwitchModel(name string) (model, provider string, err error)
}

type contextManager interface {
	ContextStatus() string
	ContextReport() string
	ContextSnapshot() commonview.ContextSnapshot
	MemoryView(scope commonview.MemoryScope) commonview.MemoryView
	CWD() string
	ClearContext()
}

type skillManager interface {
	SelectSkill(name string) error
	ClearSelectedSkill()
	SkillManagementView() commonview.SkillManagementView
	RefreshSkillManagement() commonview.SkillManagementView
	SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error)
}

type configManager interface {
	ConfigView() commonview.ConfigView
	ApplyConfig(cfg commonview.ConfigView) (commonview.ConfigView, error)
}

type sessionManager interface {
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []commonview.SessionMeta
	NewSession() (commonview.SessionMeta, error)
	DeleteSession(id string) error
}
