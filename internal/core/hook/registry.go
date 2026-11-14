package hook

import (
	"context"
	"sort"
	"sync"
)

type Kind string

const (
	Filter Kind = "filter"
	Action Kind = "action"
)

type Callback func(ctx context.Context, value any) (any, error)

type callback struct {
	owner    string
	priority int
	order    int
	fn       Callback
}

type Registry struct {
	mu      sync.RWMutex
	next    int
	filters map[string][]callback
	actions map[string][]callback
}

func NewRegistry() *Registry {
	return &Registry{
		filters: map[string][]callback{},
		actions: map[string][]callback{},
	}
}

func (r *Registry) AddFilter(name string, priority int, fn Callback) {
	r.AddFilterFor("", name, priority, fn)
}

func (r *Registry) AddFilterFor(owner, name string, priority int, fn Callback) {
	r.add(r.filters, owner, name, priority, fn)
}

func (r *Registry) AddAction(name string, priority int, fn Callback) {
	r.AddActionFor("", name, priority, fn)
}

func (r *Registry) AddActionFor(owner, name string, priority int, fn Callback) {
	r.add(r.actions, owner, name, priority, fn)
}

func (r *Registry) add(target map[string][]callback, owner, name string, priority int, fn Callback) {
	if name == "" || fn == nil {
		return
	}
	if priority == 0 {
		priority = 10
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	entry := callback{owner: owner, priority: priority, order: r.next, fn: fn}
	list := append(target[name], entry)
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].priority != list[j].priority {
			return list[i].priority < list[j].priority
		}
		return list[i].order < list[j].order
	})
	target[name] = list
}

func (r *Registry) Has(name string, kind Kind) bool {
	return r.HasFor(name, kind, "")
}

func (r *Registry) HasFor(name string, kind Kind, skipOwner string) bool {
	switch kind {
	case Filter:
		return len(r.snapshotFor(r.filters, name, skipOwner)) > 0
	case Action:
		return len(r.snapshotFor(r.actions, name, skipOwner)) > 0
	default:
		return len(r.snapshotFor(r.filters, name, skipOwner)) > 0 ||
			len(r.snapshotFor(r.actions, name, skipOwner)) > 0
	}
}

func (r *Registry) ApplyFilters(ctx context.Context, name string, value any) (any, []error) {
	return r.ApplyFiltersFrom(ctx, name, value, "")
}

func (r *Registry) ApplyFiltersFrom(ctx context.Context, name string, value any, skipOwner string) (any, []error) {
	list := r.snapshotFor(r.filters, name, skipOwner)
	current := value
	var errs []error
	for _, entry := range list {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		next, err := entry.fn(ctx, current)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if next != nil {
			current = next
		}
	}
	return current, errs
}

func (r *Registry) DoAction(ctx context.Context, name string, value any) ([]any, []error) {
	return r.DoActionFrom(ctx, name, value, "")
}

func (r *Registry) DoActionFrom(ctx context.Context, name string, value any, skipOwner string) ([]any, []error) {
	list := r.snapshotFor(r.actions, name, skipOwner)
	var outputs []any
	var errs []error
	for _, entry := range list {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		out, err := entry.fn(ctx, value)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if out != nil {
			outputs = append(outputs, out)
		}
	}
	return outputs, errs
}

func (r *Registry) snapshotFor(target map[string][]callback, name, skipOwner string) []callback {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := target[name]
	out := make([]callback, 0, len(list))
	for _, entry := range list {
		if skipOwner != "" && entry.owner == skipOwner {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filters = map[string][]callback{}
	r.actions = map[string][]callback{}
	r.next = 0
}
