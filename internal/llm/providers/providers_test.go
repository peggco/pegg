package providers

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
)

func TestGeneratedProvidersRegistered(t *testing.T) {
	for _, name := range []string{"302ai", "alibaba", "evren", "minimax-cn", "pegg-forever", "subconscious", "zenmux"} {
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

func TestSpecsRegister(t *testing.T) {
	seen := make(map[string]struct{}, len(specs))
	for _, s := range specs {
		if s.Name == "" {
			t.Fatal("spec with an empty name")
		}
		if s.BaseURL == "" && s.EnvBaseURL == "" {
			t.Fatalf("%s: spec has no base URL", s.Name)
		}
		if s.BaseURL != "" && s.EnvBaseURL != "" {
			t.Fatalf("%s: spec sets both BaseURL and EnvBaseURL", s.Name)
		}
		if _, ok := seen[s.Name]; ok {
			t.Fatalf("duplicate spec %q", s.Name)
		}
		seen[s.Name] = struct{}{}
		if !llm.HasProvider(s.Name) {
			t.Fatalf("provider %q is not registered", s.Name)
		}
	}
}

func TestSpecFactories(t *testing.T) {
	env := map[string]string{
		"CLOUDFLARE_ACCOUNT_ID":    "account",
		"DATABRICKS_HOST":          "databricks.example.com",
		"INFOMANIAK_PRODUCT_ID":    "product",
		"NEON_AI_GATEWAY_BASE_URL": "https://neon.example.com",
		"SNOWFLAKE_ACCOUNT":        "account",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}

	for _, s := range specs {
		p, err := s.newProvider(config.LLMConfig{APIKey: "test"})
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		if p.Name() != s.Name {
			t.Fatalf("%s: Name() = %q", s.Name, p.Name())
		}
	}
}

func TestSpecEnvBaseURLRequiresEnv(t *testing.T) {
	s := spec{Name: "example", EnvBaseURL: "https://${PROVIDERS_TEST_HOST}/v1"}
	t.Setenv("PROVIDERS_TEST_HOST", "")

	if _, err := s.baseURL(); err == nil || !strings.Contains(err.Error(), "PROVIDERS_TEST_HOST") {
		t.Fatalf("err = %v, want a PROVIDERS_TEST_HOST error", err)
	}
}

func TestSpecEnvBaseURLExpands(t *testing.T) {
	s := spec{Name: "example", EnvBaseURL: "https://${PROVIDERS_TEST_HOST}/v1"}
	t.Setenv("PROVIDERS_TEST_HOST", "example.test")

	got, err := s.baseURL()
	if err != nil {
		t.Fatalf("baseURL: %v", err)
	}
	if want := "https://example.test/v1"; got != want {
		t.Fatalf("baseURL = %q, want %q", got, want)
	}
}
