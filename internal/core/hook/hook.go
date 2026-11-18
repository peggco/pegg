package hook

import (
	"sync"
)

type Hook[T any] struct {
	mu      sync.RWMutex
	filters []func(T) T
}

func NewHook[T any]() *Hook[T] {
	return &Hook[T]{}
}

func (h *Hook[T]) Add(fn func(T) T) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.filters = append(h.filters, fn)
}

func (h *Hook[T]) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.filters = nil
}

func (h *Hook[T]) Has() bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.filters) > 0
}

func (h *Hook[T]) Apply(value T) T {
	h.mu.RLock()
	fns := make([]func(T) T, len(h.filters))
	copy(fns, h.filters)
	h.mu.RUnlock()

	current := value
	for _, fn := range fns {
		current = fn(current)
	}
	return current
}
