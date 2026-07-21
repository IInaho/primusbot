package runtime

import (
	"strings"
	"testing"
)

type partialCoreContextManager struct {
	snapshot ContextSnapshot
	memory   MemoryView
}

func (b *partialCoreContextManager) ContextStatus() string             { return "ok" }
func (b *partialCoreContextManager) ContextReport() string             { return "context report" }
func (b *partialCoreContextManager) ContextSnapshot() ContextSnapshot  { return b.snapshot }
func (b *partialCoreContextManager) MemoryView(MemoryScope) MemoryView { return b.memory }
func (b *partialCoreContextManager) CWD() string                       { return "/tmp/project" }
func (b *partialCoreContextManager) ClearContext()                     {}

func TestSessionRuntimeUsesPartialManagementCapabilities(t *testing.T) {
	bot := &partialCoreContextManager{
		snapshot: ContextSnapshot{Budget: 100, Used: 40},
		memory:   MemoryView{Scope: MemoryScopeProject, Path: "/tmp/memory.md"},
	}
	rt := NewSessionRuntimeWithCoreOptions(CoreSessionRuntimeOptions{
		ContextManagement: bot,
	})

	if got := rt.ContextStatus(); got != "ok" {
		t.Fatalf("ContextStatus = %q, want ok", got)
	}
	if got := rt.ContextSnapshot(); got.Budget != 100 || got.Used != 40 {
		t.Fatalf("ContextSnapshot = %#v", got)
	}
	if got := rt.MemoryView(MemoryScopeProject); got.Path != "/tmp/memory.md" {
		t.Fatalf("MemoryView = %#v", got)
	}
	if _, _, err := rt.SwitchModel("gpt"); err == nil || !strings.Contains(err.Error(), "model management") {
		t.Fatalf("SwitchModel error = %v, want model management unsupported", err)
	}
	if sessions := rt.ListSessions(); sessions != nil {
		t.Fatalf("ListSessions = %#v, want nil without session manager", sessions)
	}
}

type coreOptionRunner struct {
	configured bool
}

func (r *coreOptionRunner) ConfigureRuntime(ControlCallbacks) {
	r.configured = true
}

func (r *coreOptionRunner) Run(string, RunCallbacks) (string, error) {
	return "ok", nil
}

type coreOptionContextManager struct{}

func (coreOptionContextManager) ContextStatus() string { return "core-ok" }
func (coreOptionContextManager) ContextReport() string { return "core report" }
func (coreOptionContextManager) ContextSnapshot() ContextSnapshot {
	return ContextSnapshot{Budget: 300, Used: 30}
}
func (coreOptionContextManager) MemoryView(scope MemoryScope) MemoryView {
	return MemoryView{Scope: scope, Path: "/tmp/core-memory.md"}
}
func (coreOptionContextManager) CWD() string   { return "/tmp/core-project" }
func (coreOptionContextManager) ClearContext() {}

func TestSessionRuntimeCoreOptionsDoNotRequireViewPorts(t *testing.T) {
	runner := &coreOptionRunner{}
	rt := NewSessionRuntimeWithCoreOptions(CoreSessionRuntimeOptions{
		Runner:            runner,
		ContextManagement: coreOptionContextManager{},
	})
	if !runner.configured {
		t.Fatal("runner was not configured")
	}
	if got := rt.ContextStatus(); got != "core-ok" {
		t.Fatalf("ContextStatus = %q, want core-ok", got)
	}
	if got := rt.ContextSnapshot(); got.Budget != 300 || got.Used != 30 {
		t.Fatalf("ContextSnapshot = %#v", got)
	}
	if got := rt.MemoryView(MemoryScopeProject); got.Path != "/tmp/core-memory.md" || got.Scope != MemoryScopeProject {
		t.Fatalf("MemoryView = %#v", got)
	}
}

func TestSessionRuntimeCoreOptionsUseExplicitManagementPorts(t *testing.T) {
	manager := &partialCoreContextManager{
		snapshot: ContextSnapshot{Budget: 200, Used: 20},
		memory:   MemoryView{Scope: MemoryScopeProject, Path: "/tmp/explicit-memory.md"},
	}
	rt := NewSessionRuntimeWithCoreOptions(CoreSessionRuntimeOptions{
		ContextManagement: manager,
	})

	if got := rt.ContextStatus(); got != "ok" {
		t.Fatalf("ContextStatus = %q, want ok", got)
	}
	if got := rt.ContextSnapshot(); got.Budget != 200 || got.Used != 20 {
		t.Fatalf("ContextSnapshot = %#v", got)
	}
	if _, _, err := rt.SwitchModel("gpt"); err == nil || !strings.Contains(err.Error(), "model management") {
		t.Fatalf("SwitchModel error = %v, want model management unsupported", err)
	}
}
