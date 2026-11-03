package providers

import (
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func TestGeneratedProvidersRegistered(t *testing.T) {
	for _, name := range []string{"302ai", "alibaba", "evren", "minimax-cn", "subconscious", "zenmux"} {
		if !llm.HasProvider(name) {
			t.Fatalf("provider %q is not registered", name)
		}
	}
}

func TestProviderCount(t *testing.T) {
	if got := len(llm.ListProviders()); got < 236 {
		t.Fatalf("registered providers = %d, want at least 236", got)
	}
}
