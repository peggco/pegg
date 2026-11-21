package middleware

import (
	"sort"
	"sync"
)

var defaultRegistry = NewMiddlewareRegistry()

// Default returns the process-wide middleware catalogue populated by built-ins
// and the SDK. Agents resolve named middleware from it at run start.
func Default() *Registry {
	return defaultRegistry
}

type NamedMiddleware struct {
	Name       string
	Middleware Middleware
}

type Registry struct {
	mu   sync.RWMutex
	item map[string]Middleware
}

func NewMiddlewareRegistry() *Registry {
	return &Registry{item: make(map[string]Middleware)}
}

func (r *Registry) Register(name string, m Middleware) error {
	if m == nil {
		return ErrNilMiddleware
	}
	if name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.item[name]; ok {
		return ErrDuplicate
	}
	r.item[name] = m
	return nil
}

func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.item[name]; !ok {
		return false
	}
	delete(r.item, name)
	return true
}

func (r *Registry) Get(name string) (Middleware, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.item[name]
	return m, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.item))
	for name := range r.item {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) List() []NamedMiddleware {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]NamedMiddleware, 0, len(r.item))
	for name, m := range r.item {
		out = append(out, NamedMiddleware{Name: name, Middleware: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
