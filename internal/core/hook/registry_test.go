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

func TestRegistrySkipsOwner(t *testing.T) {
	r := NewRegistry()
	r.AddFilterFor("alpha", "providers", 10, func(ctx context.Context, value any) (any, error) {
		return "alpha", nil
	})
	r.AddFilterFor("beta", "providers", 20, func(ctx context.Context, value any) (any, error) {
		return "beta", nil
	})

	got, errs := r.ApplyFiltersFrom(context.Background(), "providers", nil, "alpha")
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if got != "beta" {
		t.Fatalf("value = %v", got)
	}

	if got, _ := r.ApplyFiltersFrom(context.Background(), "providers", nil, "beta"); got != "alpha" {
		t.Fatalf("value = %v", got)
	}

	if r.HasFor("providers", Filter, "alpha") == false {
		t.Fatal("beta handler should remain")
	}
}

func TestRegistryOnlyOwnerHasNoHandler(t *testing.T) {
	r := NewRegistry()
	r.AddActionFor("alpha", "card", 10, func(ctx context.Context, value any) (any, error) {
		return "done", nil
	})
	if r.HasFor("card", Action, "alpha") {
		t.Fatal("origin handler should be skipped")
	}
	if !r.HasFor("card", Action, "beta") {
		t.Fatal("handler should be visible to other owners")
	}
	outputs, errs := r.DoActionFrom(context.Background(), "card", nil, "alpha")
	if len(outputs) != 0 || len(errs) != 0 {
		t.Fatalf("outputs=%v errors=%v", outputs, errs)
	}
}
