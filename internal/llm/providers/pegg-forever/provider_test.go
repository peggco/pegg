package peggforever

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestNewFromConfigName(t *testing.T) {
	p, err := NewFromConfig(config.LLMConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Name(); got != ProviderName {
		t.Fatalf("Name() = %q, want %q", got, ProviderName)
	}
}

func TestNewFromConfigBaseURLOverride(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model","name":"Test Model"}]}`))
	}))
	defer srv.Close()

	p, err := NewFromConfig(config.LLMConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/models" {
		t.Fatalf("request path = %q, want %q", gotPath, "/models")
	}
	if want := "Bearer test-key"; gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
	if len(models) != 1 || models[0].ID != "test-model" {
		t.Fatalf("models = %+v, want the model served by the override URL", models)
	}
}
