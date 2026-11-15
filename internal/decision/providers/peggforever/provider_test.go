package peggforever

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/decision"
)

func TestDecideUsesForeverEndpoint(t *testing.T) {
	var gotPath, gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var req decision.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = req.Model
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decision.Response{
			Model: req.Model,
			Answers: map[string]decision.Answer{
				"q": {Type: decision.QuestionNoul, Noul: float64Ptr(0.8)},
			},
		})
	}))
	defer srv.Close()

	prov, err := NewFromConfig(config.LLMConfig{
		Provider: ProviderName,
		BaseURL:  srv.URL,
		APIKey:   "vsk_live_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := prov.Decide(context.Background(), &decision.Request{
		Questions: map[string]decision.Question{
			"q": decision.BoolQuestion("is it relevant?", nil),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != DecisionsPath {
		t.Fatalf("path = %q, want %q", gotPath, DecisionsPath)
	}
	if gotAuth != "Bearer vsk_live_test" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotModel != DefaultModel {
		t.Fatalf("model = %q, want %q", gotModel, DefaultModel)
	}
	if got, ok := resp.Answers["q"].NoulValue(); !ok || got != 0.8 {
		t.Fatalf("answer = %v, %v", got, ok)
	}
}

func TestNewFromConfigUsesDefaultBaseURL(t *testing.T) {
	prov, err := NewFromConfig(config.LLMConfig{Provider: ProviderName, APIKey: "vsk_live_test"})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != ProviderName {
		t.Fatalf("name = %q", prov.Name())
	}
}

func float64Ptr(v float64) *float64 { return &v }
