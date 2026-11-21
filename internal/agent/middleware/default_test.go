package middleware

import "testing"

func TestDefaultRegistry(t *testing.T) {
	r := Default()
	if r == nil {
		t.Fatal("Default must return a registry")
	}
	if got := Default(); got != r {
		t.Fatal("Default must return the same registry")
	}

	const name = "default-test-middleware"
	r.Unregister(name)
	defer r.Unregister(name)

	if err := r.Register(name, BaseMiddleware{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(name); !ok {
		t.Fatal("Get must return registered middleware")
	}

	found := false
	for _, n := range r.Names() {
		if n == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("names = %v, want %s", r.Names(), name)
	}
}
