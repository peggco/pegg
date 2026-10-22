package llm

import (
	"context"
	"errors"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

type declaredModelsProvider struct {
	models []Model
	err    error
}

func (p *declaredModelsProvider) Name() string { return "declared" }

func (p *declaredModelsProvider) Chat(context.Context, *Request) (*Response, error) {
	return &Response{}, nil
}

func (p *declaredModelsProvider) ChatStream(context.Context, *Request, StreamHandler) error {
	return nil
}

func (p *declaredModelsProvider) ListModels(context.Context) ([]Model, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.models, nil
}

func TestDeclaredModelsAreMergedWithDiscoveredOnes(t *testing.T) {
	mgr, _, _ := newTestManagerWithCache(t)
	RegisterProvider("declared-merge", func(cfg config.LLMConfig) (Provider, error) {
		return &declaredModelsProvider{models: []Model{
			{ID: "m1", Name: "M1"},
			{ID: "m2", Name: "M2"},
		}}, nil
	})

	mgr.Sync(context.Background(), []config.LLMConfig{{
		Provider: "declared-merge",
		Models: []config.Model{
			{ID: "m1", Name: "M1 custom", Config: &config.ModelConfig{MaxInputTokens: 4096}},
			{ID: "m3", Name: "M3", Config: &config.ModelConfig{MaxOutputTokens: 512}},
		},
	}})

	models, err := mgr.Models("declared-merge")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 {
		t.Fatalf("models = %d, want 3: %+v", len(models), models)
	}
	byID := map[string]Model{}
	for _, m := range models {
		byID[m.ID] = m
	}
	declared, ok := byID["m1"]
	if !ok {
		t.Fatal("m1 missing")
	}
	if declared.Name != "M1 custom" {
		t.Fatalf("name = %q, want the declared name", declared.Name)
	}
	if declared.Config == nil || declared.Config.MaxInputTokens != 4096 {
		t.Fatalf("config = %+v, want the declared config", declared.Config)
	}
	added, ok := byID["m3"]
	if !ok {
		t.Fatal("m3 missing")
	}
	if added.Config == nil || added.Config.MaxOutputTokens != 512 {
		t.Fatalf("config = %+v, want the declared config", added.Config)
	}
	if _, ok := byID["m2"]; !ok {
		t.Fatal("m2 must be kept")
	}
}

func TestDeclaredModelsSurviveAFailingModelList(t *testing.T) {
	mgr, _, _ := newTestManagerWithCache(t)
	RegisterProvider("declared-fail", func(cfg config.LLMConfig) (Provider, error) {
		return &declaredModelsProvider{err: errors.New("no models endpoint")}, nil
	})

	mgr.Sync(context.Background(), []config.LLMConfig{{
		Provider: "declared-fail",
		Models:   []config.Model{{ID: "m1", Name: "M1"}},
	}})

	models, err := mgr.Models("declared-fail")
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	if len(models) != 1 || models[0].ID != "m1" {
		t.Fatalf("models = %+v, want the declared model", models)
	}
}

func TestDeclaredModelsAreWrittenToTheCache(t *testing.T) {
	mgr, _, store := newTestManagerWithCache(t)
	RegisterProvider("declared-cache", func(cfg config.LLMConfig) (Provider, error) {
		return &declaredModelsProvider{err: errors.New("no models endpoint")}, nil
	})

	mgr.Sync(context.Background(), []config.LLMConfig{{
		Provider: "declared-cache",
		Models:   []config.Model{{ID: "m1", Name: "M1"}},
	}})

	raw, err := store.Get("declared-cache")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	var models []Model
	if err := json.Unmarshal(raw, &models); err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "m1" {
		t.Fatalf("cached models = %+v", models)
	}
}

func TestDeclaredModelWithoutConfigKeepsDiscoveredMetadata(t *testing.T) {
	mgr, _, _ := newTestManagerWithCache(t)
	RegisterProvider("declared-keep", func(cfg config.LLMConfig) (Provider, error) {
		return &declaredModelsProvider{models: []Model{
			{ID: "m1", Name: "M1", Config: &ModelConfig{MaxInputTokens: 2048}},
		}}, nil
	})

	mgr.Sync(context.Background(), []config.LLMConfig{{
		Provider: "declared-keep",
		Models:   []config.Model{{ID: "m1"}},
	}})

	models, err := mgr.Models("declared-keep")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %+v", models)
	}
	if models[0].Config == nil || models[0].Config.MaxInputTokens != 2048 {
		t.Fatalf("config = %+v, want the discovered metadata", models[0].Config)
	}
	if models[0].Name != "M1" {
		t.Fatalf("name = %q, want the discovered name", models[0].Name)
	}
}

func TestDeclaredModelReasoningOptionsImplySupportsReasoning(t *testing.T) {
	mgr, _, _ := newTestManagerWithCache(t)
	RegisterProvider("declared-reasoning", func(cfg config.LLMConfig) (Provider, error) {
		return &declaredModelsProvider{err: errors.New("no models endpoint")}, nil
	})

	mgr.Sync(context.Background(), []config.LLMConfig{{
		Provider: "declared-reasoning",
		Models: []config.Model{{
			ID: "r1",
			Config: &config.ModelConfig{
				ReasoningOptions: []config.ReasoningOption{{Type: "effort", Values: []string{"low", "medium", "high"}}},
			},
		}},
	}})

	models, err := mgr.Models("declared-reasoning")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %+v", models)
	}
	if models[0].Config == nil || !models[0].Config.SupportsReasoning {
		t.Fatalf("config = %+v, want supports_reasoning", models[0].Config)
	}
	efforts := models[0].Config.ReasoningEfforts()
	if len(efforts) != 3 || efforts[2] != "high" {
		t.Fatalf("efforts = %v", efforts)
	}
}
