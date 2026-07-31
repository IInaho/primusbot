package catalog

import (
	"nekocode/bot/config"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/shell"
)

// Toolbox is the assembled builtin tool environment: a registry with the
// full catalog plus the stateful shell tool instance, which is kept alive
// across registry rebuilds so shutdown and sandbox wiring survive.
type Toolbox struct {
	Registry *tools.Registry
	shell    *shell.ShellTool
}

// NewToolbox assembles a fresh registry with the full catalog.
func NewToolbox(imageGen []config.ImageGenConfig) *Toolbox {
	e := &Toolbox{}
	e.RebuildRegistry(imageGen)
	return e
}

// RebuildRegistry assembles a fresh registry with the full catalog. The
// stateful shell tool instance is kept alive across rebuilds: it replaces
// the freshly registered one so its shutdown state and sandbox wiring are
// preserved.
func (e *Toolbox) RebuildRegistry(imageGen []config.ImageGenConfig) {
	existing := e.shell
	e.Registry = tools.New()
	RegisterAll(e.Registry, imageGen)
	if existing != nil {
		e.Registry.Register(existing)
		return
	}
	if t, err := e.Registry.Get("shell"); err == nil {
		if sh, ok := t.(*shell.ShellTool); ok {
			e.shell = sh
		}
	}
}

// Shutdown shuts down the stateful shell tool. It is nil-safe and
// idempotent: a second call is a no-op.
func (e *Toolbox) Shutdown() {
	if e.shell == nil {
		return
	}
	e.shell.Shutdown()
	e.shell = nil
}

// SetSandboxProfiler injects the permission engine into the shell tool.
// Nil-safe: a no-op when the shell tool is gone (e.g. after Shutdown).
func (e *Toolbox) SetSandboxProfiler(p shell.SandboxProfiler) {
	if e.shell != nil {
		e.shell.SetSandboxProfiler(p)
	}
}
