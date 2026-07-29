package skill

import (
	"strings"
	"sync"

	utilregistry "nekocode/util/registry"
)

// registry manages loaded skills, thread-safe.
type registry struct {
	*utilregistry.Registry[*Skill]
	loaded sync.Map
}

func newRegistry() *registry {
	return &registry{
		Registry: utilregistry.New[*Skill](func(s *Skill) string { return s.Name }),
	}
}

// Load discovers and registers skills under dirs. Skills that fail to parse
// are skipped; already-registered names win.
func (r *registry) Load(dirs []string) {
	for _, p := range discoverSkills(dirs) {
		sk, err := loadSkill(p)
		if err != nil {
			continue
		}
		if !r.Has(sk.Name) {
			r.Register(sk)
		}
	}
}

func (r *registry) MarkLoaded(name string) {
	r.loaded.Store(name, true)
}

func (r *registry) ClearLoaded() {
	r.loaded.Clear()
}

func (r *registry) IsLoaded(name string) bool {
	_, ok := r.loaded.Load(name)
	return ok
}

func (r *registry) LoadedSet() map[string]bool {
	out := make(map[string]bool)
	r.loaded.Range(func(key, value any) bool {
		out[key.(string)] = true
		return true
	})
	return out
}

func (r *registry) namesString() string {
	names := r.Names()
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
