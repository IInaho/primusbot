package tools

import (
	"nekocode/bot/tools/runtime/core"
	"nekocode/util/registry"
)

// Registry is a thread-safe tool registry backed by a generic registry.
type Registry struct {
	*registry.Registry[core.Tool]
}

// New creates a registry containing the supplied tools.
func New(items ...core.Tool) *Registry {
	r := &Registry{
		Registry: registry.New[core.Tool](func(t core.Tool) string { return t.Name() }),
	}
	r.RegisterAll(items)
	return r
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
