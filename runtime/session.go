package runtime

import "nekocode/runtime/internal/session"

type SessionRuntime struct {
	*session.SessionRuntime
	modelManagement   CoreModelManager
	contextManagement CoreContextManager
	skillManagement   CoreSkillManager
	configManagement  CoreConfigManager
	sessionManagement CoreSessionManager
}

var _ Runtime = (*SessionRuntime)(nil)
var _ QueryRuntime = (*SessionRuntime)(nil)
var _ ManagementRuntime = (*SessionRuntime)(nil)

func NewSessionRuntimeWithCoreOptions(opts CoreSessionRuntimeOptions) *SessionRuntime {
	return &SessionRuntime{
		SessionRuntime:    session.NewSessionRuntimeWithPorts(coreSessionPortsFromOptions(opts)),
		modelManagement:   opts.ModelManagement,
		contextManagement: opts.ContextManagement,
		skillManagement:   opts.SkillManagement,
		configManagement:  opts.ConfigManagement,
		sessionManagement: opts.SessionManagement,
	}
}
