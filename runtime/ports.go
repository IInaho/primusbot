package runtime

import "nekocode/runtime/internal/session"

type CoreAgentRunner interface {
	Run(input string, callbacks RunCallbacks) (string, error)
	ConfigureRuntime(callbacks ControlCallbacks)
}

type CoreCommandExecutor interface {
	ExecuteCommand(input string) (string, CmdResult)
}

type CoreSkillHintProvider interface {
	SkillHint() (string, bool)
}

type CoreCommandCatalog interface {
	CommandNames() []string
}

type CoreRunController interface {
	Steer(msg string)
	Abort()
	Close()
}

type CoreStatsProvider interface {
	Stats() BotStats
}

type CoreModelInfoProvider interface {
	ProviderModel() (provider, model string)
}

type CoreMessageProvider interface {
	SessionMessages() []DisplayMessage
}

type CoreModelManager interface {
	SwitchModel(name string) (model, provider string, err error)
}

type CoreContextManager interface {
	ContextStatus() string
	ContextReport() string
	ContextSnapshot() ContextSnapshot
	MemoryView(scope MemoryScope) MemoryView
	CWD() string
	ClearContext()
}

type CoreSkillManager interface {
	SelectSkill(name string) error
	ClearSelectedSkill()
	SkillManagementView() SkillManagementView
	RefreshSkillManagement() SkillManagementView
	SetPluginEnabled(name string, enabled bool) (SkillManagementView, error)
}

type CoreConfigManager interface {
	ConfigView() ConfigView
	ApplyConfig(cfg ConfigView) (ConfigView, error)
}

type CoreSessionManager interface {
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []SessionMeta
	NewSession() (SessionMeta, error)
	DeleteSession(id string) error
}

type CoreSessionRuntimeOptions struct {
	Runner   CoreAgentRunner
	Commands CoreCommandExecutor
	Skills   CoreSkillHintProvider
	Catalog  CoreCommandCatalog
	Control  CoreRunController
	Stats    CoreStatsProvider
	Model    CoreModelInfoProvider
	Messages CoreMessageProvider

	ModelManagement   CoreModelManager
	ContextManagement CoreContextManager
	SkillManagement   CoreSkillManager
	ConfigManagement  CoreConfigManager
	SessionManagement CoreSessionManager
}

func coreSessionPortsFromOptions(opts CoreSessionRuntimeOptions) session.Ports {
	return session.Ports{
		Runner:   opts.Runner,
		Commands: opts.Commands,
		Skills:   opts.Skills,
		Catalog:  opts.Catalog,
		Control:  opts.Control,
		Stats:    opts.Stats,
		Model:    opts.Model,
		Messages: opts.Messages,
	}
}
