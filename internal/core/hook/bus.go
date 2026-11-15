package hook

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync/atomic"
	"time"
)

const defaultTimeout = 5 * time.Second

type Bus struct {
	registry *Registry
	timeout  atomic.Int64
	enabled  atomic.Bool
	disabled atomic.Pointer[map[string]struct{}]
}

func NewBus() *Bus {
	b := &Bus{registry: NewRegistry()}
	b.timeout.Store(int64(defaultTimeout))
	b.enabled.Store(true)
	empty := map[string]struct{}{}
	b.disabled.Store(&empty)
	return b
}

var defaultBus atomic.Pointer[Bus]

func init() {
	defaultBus.Store(NewBus())
}

func Default() *Bus {
	return defaultBus.Load()
}

func SetDefault(b *Bus) {
	if b == nil {
		b = NewBus()
	}
	defaultBus.Store(b)
}

func ResetDefault() {
	defaultBus.Store(NewBus())
}

func (b *Bus) Registry() *Registry {
	return b.registry
}

func (b *Bus) Timeout() time.Duration {
	timeout := time.Duration(b.timeout.Load())
	if timeout <= 0 {
		return defaultTimeout
	}
	return timeout
}

func (b *Bus) Configure(enabled bool, disabled []string, timeout time.Duration) {
	b.enabled.Store(enabled)
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	b.timeout.Store(int64(timeout))
	set := make(map[string]struct{}, len(disabled))
	for _, name := range disabled {
		if name = strings.TrimSpace(name); name != "" {
			set[name] = struct{}{}
		}
	}
	b.disabled.Store(&set)
}

func (b *Bus) Allowed(name string) bool {
	if !b.enabled.Load() {
		return false
	}
	set := b.disabled.Load()
	if set == nil || len(*set) == 0 {
		return true
	}
	_, blocked := (*set)[name]
	return !blocked
}

func (b *Bus) HasFilter(name string) bool {
	return b.Allowed(name) && b.registry.Has(name, Filter)
}

func (b *Bus) HasAction(name string) bool {
	return b.Allowed(name) && b.registry.Has(name, Action)
}

func HasFilter(name string) bool {
	return Default().HasFilter(name)
}

func HasAction(name string) bool {
	return Default().HasAction(name)
}

func (b *Bus) context(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, b.Timeout())
}

func (b *Bus) applyValue(ctx context.Context, name string, value any) (any, []error) {
	callCtx, cancel := b.context(ctx)
	defer cancel()
	return b.registry.ApplyFilters(callCtx, name, value)
}

func ApplyFilter[T any](ctx context.Context, name string, value T) (T, []error) {
	b := Default()
	if !b.Allowed(name) || !b.registry.Has(name, Filter) {
		return value, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value, []error{err}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return value, []error{err}
	}
	var original any
	if err := json.Unmarshal(raw, &original); err != nil {
		return value, []error{err}
	}
	out, errs := b.applyValue(ctx, name, decoded)
	if reflect.DeepEqual(original, out) {
		return value, errs
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return value, append(errs, err)
	}
	var typed T
	if err := json.Unmarshal(encoded, &typed); err != nil {
		return value, append(errs, err)
	}
	return typed, errs
}

func ApplyAction(ctx context.Context, name string, value any) ([]json.RawMessage, []error) {
	b := Default()
	if !b.Allowed(name) || !b.registry.Has(name, Action) {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, []error{err}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, []error{err}
	}
	callCtx, cancel := b.context(ctx)
	defer cancel()
	outputs, errs := b.registry.DoAction(callCtx, name, decoded)
	out := make([]json.RawMessage, 0, len(outputs))
	for _, output := range outputs {
		encoded, err := json.Marshal(output)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, encoded)
	}
	return out, errs
}
