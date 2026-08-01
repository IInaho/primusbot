package catalog

import (
	"nekocode/bot/config"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/runtime/workspace"
)

// Toolbox owns the builtin registry and the stateful shell runtime shared by
// shell and process across registry rebuilds.
type Toolbox struct {
	Registry  *tools.Registry
	shell     *shell.ShellTool
	workspace *workspace.Manager
}

// NewToolbox assembles a fresh registry with the full catalog.
func NewToolbox(imageGen []config.ImageGenConfig) *Toolbox {
	e := &Toolbox{workspace: workspace.New("", nil)}
	e.RebuildRegistry(imageGen)
	return e
}

// RebuildRegistry keeps the shell runtime alive while replacing tool schemas
// and configuration-dependent tools.
func (e *Toolbox) RebuildRegistry(imageGen []config.ImageGenConfig) {
	if e.shell == nil {
		e.shell = &shell.ShellTool{}
	}
	if e.workspace == nil {
		e.workspace = workspace.New("", nil)
	}
	e.Registry = tools.New()
	e.Registry.SetWorkspace(e.workspace)
	registerAll(e.Registry, imageGen, e.shell)
}

func (e *Toolbox) Workspace() *workspace.Manager { return e.workspace }

// ManagedProcessSummary returns volatile process state for the next model
// request. It is not persisted in conversation history.
func (e *Toolbox) ManagedProcessSummary() string {
	if e.shell == nil {
		return ""
	}
	return e.shell.ProcessSummary()
}

func (e *Toolbox) SetSessionID(id string) {
	if e.workspace != nil {
		e.workspace.SetSession(id)
	}
	if e.shell != nil {
		e.shell.SetSessionID(id)
	}
}

// CloseSession stops owned processes before revoking temporary workspace
// grants. On failure, callers must keep the current session active.
func (e *Toolbox) CloseSession(id string) error {
	if e.shell != nil {
		if err := e.shell.StopSession(id); err != nil {
			return err
		}
	}
	if e.workspace != nil {
		e.workspace.DropSession(id)
	}
	return nil
}

// Close shuts down the stateful shell tool. It is nil-safe and
// idempotent: a second call is a no-op.
func (e *Toolbox) Close() error {
	if e.shell == nil {
		return nil
	}
	if err := e.shell.Shutdown(); err != nil {
		return err
	}
	e.shell = nil
	return nil
}

// SetSandboxProfiler injects the permission engine into the shell tool.
// Nil-safe: a no-op when the shell tool is gone (e.g. after Close).
func (e *Toolbox) SetSandboxProfiler(p shell.SandboxProfiler) {
	if e.shell != nil {
		e.shell.SetSandboxProfiler(p)
	}
}
