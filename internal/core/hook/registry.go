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
	r.add(r.filters, name, priority, fn)
}

func (r *Registry) AddAction(name string, priority int, fn Callback) {
	r.add(r.actions, name, priority, fn)
}

func (r *Registry) add(target map[string][]callback, name string, priority int, fn Callback) {
	if name == "" || fn == nil {
		return
	}
	if priority == 0 {
		priority = 10
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	entry := callback{priority: priority, order: r.next, fn: fn}
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	switch kind {
	case Filter:
		return len(r.filters[name]) > 0
	case Action:
		return len(r.actions[name]) > 0
	default:
		return len(r.filters[name]) > 0 || len(r.actions[name]) > 0
	}
}

func (r *Registry) ApplyFilters(ctx context.Context, name string, value any) (any, []error) {
	list := r.snapshot(r.filters, name)
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
	list := r.snapshot(r.actions, name)
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

func (r *Registry) snapshot(target map[string][]callback, name string) []callback {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := target[name]
	out := make([]callback, len(list))
	copy(out, list)
	return out
}

func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filters = map[string][]callback{}
	r.actions = map[string][]callback{}
	r.next = 0
}
