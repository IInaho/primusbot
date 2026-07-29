// Package runtime provides the interaction control layer above the Bot core.
//
// The package entry point is SessionRuntime, constructed with
// NewSessionRuntimeWithCoreOptions: the caller (see runtime/defaultbot for
// the production wiring) injects the bot's capabilities as small interfaces
// (ports.go), and SessionRuntime fans them out to the three public contract
// faces — Runtime (event-driven run control), QueryRuntime (read models),
// and ManagementRuntime (optional management capabilities, management.go).
// All DTO types are aliases of runtime/internal/core (protocol.go);
// internal/ holds the machinery (session orchestration, event bus, brokers,
// run store, connectors) and is not importable outside this module.
package runtime

import "nekocode/runtime/internal/session"

// SessionRuntime is the package entry point: it composes the internal
// session orchestrator with the optional management capabilities.
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
