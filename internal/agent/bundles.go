package agent

import (
	"sort"

	"github.com/peggco/pegg/internal/agent/tool"
)

func (a *Agent) ActivateBundle(name string, ts ...tool.Tool) []string {
	if a == nil || name == "" {
		return nil
	}

	a.bundlesMu.Lock()
	defer a.bundlesMu.Unlock()

	if a.activeBundles == nil {
		a.activeBundles = make(map[string][]tool.Tool)
	}
	existing := a.activeBundles[name]
	seen := make(map[string]bool, len(existing)+len(ts))
	for _, t := range existing {
		if t != nil {
			seen[t.Name()] = true
		}
	}

	added := make([]string, 0, len(ts))
	for _, t := range ts {
		if t == nil || t.Name() == "" || seen[t.Name()] {
			continue
		}
		if current, ok := a.Tools.Get(t.Name()); ok {
			existing = append(existing, current)
			seen[t.Name()] = true
			continue
		}
		if err := a.Tools.Register(t); err != nil {
			continue
		}
		existing = append(existing, t)
		seen[t.Name()] = true
		added = append(added, t.Name())
	}
	if len(existing) > 0 {
		a.activeBundles[name] = existing
	}
	return added
}

func (a *Agent) BundleNames() []string {
	if a == nil {
		return nil
	}
	a.bundlesMu.RLock()
	defer a.bundlesMu.RUnlock()
	out := make([]string, 0, len(a.activeBundles))
	for name := range a.activeBundles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (a *Agent) InheritBundles(parent *Agent) {
	if a == nil || parent == nil {
		return
	}
	parent.bundlesMu.RLock()
	snapshot := make(map[string][]tool.Tool, len(parent.activeBundles))
	for name, ts := range parent.activeBundles {
		snapshot[name] = append([]tool.Tool(nil), ts...)
	}
	parent.bundlesMu.RUnlock()
	for name, ts := range snapshot {
		a.ActivateBundle(name, ts...)
	}
}
