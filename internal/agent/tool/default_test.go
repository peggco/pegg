package tool

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	r := Default()
	if r == nil {
		t.Fatal("Default must return a registry")
	}
	if got := Default(); got != r {
		t.Fatal("Default must return the same registry")
	}

	const name = "default-test-tool"
	r.Unregister(name)
	defer r.Unregister(name)

	err := r.Register(NewSpec(name, "test", nil, func(context.Context, string) (string, error) {
		return "x", nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	names := r.Names()
	found := false
	for _, n := range names {
		if n == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("names = %v, want %s", names, name)
	}

	if _, ok := r.Get(name); !ok {
		t.Fatal("Get must return registered tool")
	}

	llmTools := r.LLMTools()
	found = false
	for _, lt := range llmTools {
		if lt.Function.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s missing from LLMTools: %+v", name, llmTools)
	}
}

func TestDefaultRegistryDuplicate(t *testing.T) {
	r := Default()
	const name = "default-test-tool"
	r.Unregister(name)
	defer r.Unregister(name)

	spec := NewSpec(name, "test", nil, func(context.Context, string) (string, error) {
		return "x", nil
	})
	if err := r.Register(spec); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(spec); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}
}
