package hook

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func withBus(t *testing.T) *Bus {
	t.Helper()
	bus := NewBus()
	SetDefault(bus)
	t.Cleanup(ResetDefault)
	return bus
}

type busPayload struct {
	Name    string `json:"name"`
	Content any    `json:"content"`
}

func TestBusFilterFastPathSkipsSerialization(t *testing.T) {
	withBus(t)
	unserializable := make(chan int)
	out, errs := ApplyFilter(context.Background(), "test.fast", unserializable)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if out != unserializable {
		t.Fatal("value changed")
	}
}

func TestBusFilterRewritesStruct(t *testing.T) {
	bus := withBus(t)
	bus.Registry().AddFilterFor("writer", "test.rewrite", 10, func(ctx context.Context, value any) (any, error) {
		payload, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("payload: %T", value)
		}
		payload["name"] = "changed"
		return payload, nil
	})
	out, errs := ApplyFilter(context.Background(), "test.rewrite", busPayload{Name: "original"})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if out.Name != "changed" {
		t.Fatalf("name: %q", out.Name)
	}
}

func TestBusFilterUnchangedKeepsConcreteType(t *testing.T) {
	bus := withBus(t)
	bus.Registry().AddFilterFor("observer", "test.observe", 10, func(ctx context.Context, value any) (any, error) {
		return value, nil
	})
	type content struct{ Value int }
	out, errs := ApplyFilter(context.Background(), "test.observe", busPayload{Name: "n", Content: content{Value: 3}})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if _, ok := out.Content.(content); !ok {
		t.Fatalf("content type: %T", out.Content)
	}
}

func TestBusDisabledSkipsHandler(t *testing.T) {
	bus := withBus(t)
	called := false
	bus.Registry().AddFilterFor("writer", "test.disabled", 10, func(ctx context.Context, value any) (any, error) {
		called = true
		return value, nil
	})
	bus.Configure(true, []string{"test.disabled"}, 0)
	if _, errs := ApplyFilter(context.Background(), "test.disabled", "x"); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if called {
		t.Fatal("disabled handler ran")
	}
	bus.Configure(false, nil, 0)
	if _, errs := ApplyFilter(context.Background(), "test.disabled", "x"); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if called {
		t.Fatal("handler ran while bus disabled")
	}
	bus.Configure(true, nil, 0)
	if _, errs := ApplyFilter(context.Background(), "test.disabled", "x"); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if !called {
		t.Fatal("handler did not run after re-enable")
	}
}

func TestBusActionCollectsOutputs(t *testing.T) {
	bus := withBus(t)
	bus.Registry().AddActionFor("writer", "test.action", 10, func(ctx context.Context, value any) (any, error) {
		return "one", nil
	})
	bus.Registry().AddActionFor("writer", "test.action", 20, func(ctx context.Context, value any) (any, error) {
		return nil, errors.New("boom")
	})
	outputs, errs := ApplyAction(context.Background(), "test.action", map[string]bool{"ok": true})
	if len(outputs) != 1 {
		t.Fatalf("outputs: %#v", outputs)
	}
	var decoded string
	if err := json.Unmarshal(outputs[0], &decoded); err != nil || decoded != "one" {
		t.Fatalf("output: %s err=%v", outputs[0], err)
	}
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Fatalf("errors: %v", errs)
	}
}

func TestBusTimeoutBoundsHandlers(t *testing.T) {
	bus := withBus(t)
	bus.Configure(true, nil, 20*time.Millisecond)
	bus.Registry().AddFilterFor("writer", "test.slow", 10, func(ctx context.Context, value any) (any, error) {
		<-ctx.Done()
		return value, ctx.Err()
	})
	start := time.Now()
	_, errs := ApplyFilter(context.Background(), "test.slow", "x")
	if len(errs) == 0 {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("hook took %s", elapsed)
	}
}

func TestRegistryRemoveOwner(t *testing.T) {
	bus := withBus(t)
	bus.Registry().AddFilterFor("alpha", "shared", 10, func(ctx context.Context, value any) (any, error) {
		return "alpha", nil
	})
	bus.Registry().AddFilterFor("beta", "shared", 20, func(ctx context.Context, value any) (any, error) {
		return "beta", nil
	})
	bus.Registry().RemoveOwner("alpha")
	out, errs := ApplyFilter(context.Background(), "shared", "x")
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if out != "beta" {
		t.Fatalf("value: %v", out)
	}
	bus.Registry().RemoveOwner("beta")
	if bus.Registry().Has("shared", Filter) {
		t.Fatal("owner handlers remain")
	}
}
