package tools

import (
	"sync"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/util/registry"
)

// Registry is a thread-safe tool registry backed by a generic registry.
type Registry struct {
	*registry.Registry[core.Tool]
	mu        sync.RWMutex
	workspace *workspace.Manager
	planTools map[string]struct{}
}

// New creates a registry containing the supplied tools.
func New(items ...core.Tool) *Registry {
	r := &Registry{
		Registry:  registry.New[core.Tool](func(t core.Tool) string { return t.Name() }),
		workspace: workspace.New("", nil),
		planTools: make(map[string]struct{}),
	}
	r.RegisterAll(items)
	return r
}

// Workspace returns the authority shared by this registry's executor and
// tools. Each registry owns an independent instance.
func (r *Registry) Workspace() *workspace.Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.workspace
}

func (r *Registry) SetWorkspace(manager *workspace.Manager) {
	if manager != nil {
		r.mu.Lock()
		r.workspace = manager
		r.mu.Unlock()
	}
}

// AllowInPlan declares tools that may execute while plan mode is active.
func (r *Registry) AllowInPlan(names ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range names {
		if name != "" {
			r.planTools[name] = struct{}{}
		}
	}
}

func (r *Registry) PlanAllows(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.planTools[name]
	return ok
}

// Get returns a tool by name, or an error if not found.
func (r *Registry) Get(name string) (core.Tool, error) {
	return r.GetOrError(name, "tool not found: %s")
}

// Descriptors returns tool descriptors for all registered tools, sorted by name.
func (r *Registry) Descriptors() []core.Descriptor {
	list := r.List()
	descs := make([]core.Descriptor, len(list))
	for i, t := range list {
		descs[i] = core.Descriptor{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
	}
	return descs
}
