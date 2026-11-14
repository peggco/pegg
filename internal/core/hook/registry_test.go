package hook

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryFiltersChainByPriority(t *testing.T) {
	r := NewRegistry()
	r.AddFilter("value", 20, func(ctx context.Context, value any) (any, error) {
		return value.(string) + "-late", nil
	})
	r.AddFilter("value", 10, func(ctx context.Context, value any) (any, error) {
		return value.(string) + "-early", nil
	})
	got, errs := r.ApplyFilters(context.Background(), "value", "start")
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if got != "start-early-late" {
		t.Fatalf("value = %q", got)
	}
}

func TestRegistryActionsRunInOrderAndCollectErrors(t *testing.T) {
	r := NewRegistry()
	var order []string
	r.AddAction("act", 10, func(ctx context.Context, value any) (any, error) {
		order = append(order, "first")
		return "one", nil
	})
	r.AddAction("act", 10, func(ctx context.Context, value any) (any, error) {
		order = append(order, "second")
		return nil, errors.New("boom")
	})
	r.AddAction("act", 20, func(ctx context.Context, value any) (any, error) {
		order = append(order, "third")
		return "three", nil
	})
	outputs, errs := r.DoAction(context.Background(), "act", nil)
	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("order = %v", order)
	}
	if len(outputs) != 2 || outputs[0] != "one" || outputs[1] != "three" {
		t.Fatalf("outputs = %#v", outputs)
	}
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Fatalf("errors = %v", errs)
	}
}

func TestRegistrySeparatesFiltersAndActions(t *testing.T) {
	r := NewRegistry()
	r.AddFilter("same", 10, func(ctx context.Context, value any) (any, error) { return value, nil })
	if !r.Has("same", Filter) || r.Has("same", Action) {
		t.Fatalf("kind separation failed")
	}
	r.Reset()
	if r.Has("same", Filter) {
		t.Fatalf("reset did not clear hooks")
	}
}
