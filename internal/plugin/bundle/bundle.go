package bundle

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/vfs"
)

type Deps struct {
	FS *vfs.VFS
}

type Build func(Deps) ([]tool.Tool, error)

var (
	mu       sync.RWMutex
	builders = map[string][]Build{}
)

func Register(name string, b Build) error {
	if name == "" {
		return fmt.Errorf("tool bundle name is required")
	}
	if b == nil {
		return fmt.Errorf("tool bundle %q has no builder", name)
	}
	mu.Lock()
	builders[name] = append(builders[name], b)
	mu.Unlock()
	return nil
}

func RegisterTools(name string, ts []tool.Tool) error {
	if len(ts) == 0 {
		return nil
	}
	return Register(name, func(Deps) ([]tool.Tool, error) {
		return ts, nil
	})
}

func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(builders))
	for name := range builders {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func Activate(ctx context.Context, deps Deps, names []string) []string {
	if len(names) == 0 {
		return nil
	}
	a := agent.FromContext(ctx)
	if a == nil {
		return []string{"cannot activate tool bundles: no active agent in context"}
	}

	status := make([]string, 0, len(names))
	for _, name := range names {
		ts, err := build(name, deps)
		if err != nil {
			status = append(status, fmt.Sprintf("tool bundle %q: %v", name, err))
			continue
		}
		if len(ts) == 0 {
			status = append(status, fmt.Sprintf("tool bundle %q is not available", name))
			continue
		}
		added := a.ActivateBundle(name, ts...)
		if len(added) == 0 {
			status = append(status, fmt.Sprintf("tool bundle %q is already active", name))
			continue
		}
		status = append(status, fmt.Sprintf("Activated tool bundle: %s (%d tools).", name, len(added)))
	}
	return status
}

func build(name string, deps Deps) ([]tool.Tool, error) {
	mu.RLock()
	list := append([]Build(nil), builders[name]...)
	mu.RUnlock()
	if len(list) == 0 {
		return nil, nil
	}

	seen := map[string]bool{}
	out := make([]tool.Tool, 0, 8)
	for _, build := range list {
		ts, err := build(deps)
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			if t == nil || t.Name() == "" || seen[t.Name()] {
				continue
			}
			seen[t.Name()] = true
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}
