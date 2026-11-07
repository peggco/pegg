package agents

import (
	"fmt"
	"sort"
	"sync"

	"github.com/peggco/pegg/internal/agent"
)

type Factory func() (*agent.Agent, error)

var (
	mu           sync.RWMutex
	factories    = make(map[string]Factory)
	descriptions = make(map[string]string)
)

type Info struct {
	Name        string
	Description string
}

func Register(f Factory) error {
	if f == nil {
		return ErrNilFactory
	}
	a, err := f()
	if err != nil {
		return fmt.Errorf("agents: register: %w", err)
	}
	if a == nil {
		return ErrNilAgent
	}
	if a.Name == "" {
		return ErrEmptyName
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := factories[a.Name]; ok {
		return fmt.Errorf("%w: %q", ErrDuplicate, a.Name)
	}
	factories[a.Name] = f
	descriptions[a.Name] = a.Description
	return nil
}

func ListInfo() []Info {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Info, 0, len(factories))
	for name := range factories {
		out = append(out, Info{Name: name, Description: descriptions[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Has(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := factories[name]
	return ok
}

func New(name string) (*agent.Agent, error) {
	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return f()
}
