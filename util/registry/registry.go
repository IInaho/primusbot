// Package registry provides a thread-safe named-item registry.
package registry

import (
	"fmt"
	"sort"
	"sync"
)

type Registry[T any] struct {
	mu     sync.RWMutex
	items  map[string]T
	nameFn func(T) string
}

func New[T any](nameFn func(T) string) *Registry[T] {
	return &Registry[T]{
		items:  make(map[string]T),
		nameFn: nameFn,
	}
}

func (r *Registry[T]) Register(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[r.nameFn(item)] = item
}

func (r *Registry[T]) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, name)
}

func (r *Registry[T]) Get(name string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[name]
	return t, ok
}

func (r *Registry[T]) GetOrError(name string, formatMsg string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[name]
	if !ok {
		var zero T
		return zero, fmt.Errorf(formatMsg, name)
	}
	return t, nil
}

func (r *Registry[T]) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.items[name]
	return ok
}

func (r *Registry[T]) List() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := r.sortedNames()
	out := make([]T, len(names))
	for i, n := range names {
		out[i] = r.items[n]
	}
	return out
}

func (r *Registry[T]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sortedNames()
}

func (r *Registry[T]) RegisterAll(items []T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		name := r.nameFn(item)
		if _, exists := r.items[name]; !exists {
			r.items[name] = item
		}
	}
}

func (r *Registry[T]) sortedNames() []string {
	names := make([]string, 0, len(r.items))
	for n := range r.items {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
