package runtime

import (
	"strings"
	"testing"
)

type contextBackend struct {
	testBot
	snapshot ContextSnapshot
	memory   MemoryView
}

func (b *contextBackend) ContextStatus() string             { return "ok" }
func (b *contextBackend) ContextReport() string             { return "context report" }
func (b *contextBackend) ContextSnapshot() ContextSnapshot  { return b.snapshot }
func (b *contextBackend) MemoryView(MemoryScope) MemoryView { return b.memory }
func (b *contextBackend) CWD() string                       { return "/tmp/project" }
func (b *contextBackend) ClearContext()                     {}

func TestManagerUsesCapabilitiesFromBackend(t *testing.T) {
	backend := &contextBackend{
		snapshot: ContextSnapshot{Budget: 100, Used: 40},
		memory:   MemoryView{Scope: MemoryScopeProject, Path: "/tmp/memory.md"},
	}
	rt := New(backend)

	if got := rt.ContextStatus(); got != "ok" {
		t.Fatalf("ContextStatus = %q, want ok", got)
	}
	if got := rt.ContextSnapshot(); got.Budget != 100 || got.Used != 40 {
		t.Fatalf("ContextSnapshot = %#v", got)
	}
	if got := rt.MemoryView(MemoryScopeProject); got.Path != "/tmp/memory.md" {
		t.Fatalf("MemoryView = %#v", got)
	}
}

func TestManagerReportsUnsupportedOptionalCapabilities(t *testing.T) {
	rt := New(&testBot{})

	if _, _, err := rt.SwitchModel("gpt"); err == nil || !strings.Contains(err.Error(), "model management") {
		t.Fatalf("SwitchModel error = %v, want model management unsupported", err)
	}
	if sessions := rt.ListSessions(); sessions != nil {
		t.Fatalf("ListSessions = %#v, want nil without session manager", sessions)
	}
}

func TestNewRejectsNilBackend(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New(nil) did not panic")
		}
	}()
	New(nil)
}
