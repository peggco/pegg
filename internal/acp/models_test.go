package acp

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

type listModelsProvider struct {
	name   string
	models []llm.Model
}

func (p *listModelsProvider) Name() string { return p.name }

func (p *listModelsProvider) Chat(context.Context, *llm.Request) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (p *listModelsProvider) ChatStream(context.Context, *llm.Request, llm.StreamHandler) error {
	return nil
}

func (p *listModelsProvider) ListModels(context.Context) ([]llm.Model, error) {
	return p.models, nil
}

func TestHandleListModelsReloadsConfiguredProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const providerName = "acp-reload-test"
	llm.RegisterProvider(providerName, func(config.LLMConfig) (llm.Provider, error) {
		return &listModelsProvider{name: providerName, models: []llm.Model{{ID: "team-model", Name: "Team Model"}}}, nil
	})

	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{{Provider: providerName}}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	bus := event.New()
	log := logger.New(logger.LevelError)
	mgr := llm.NewManager(bus, log, nil)
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Shutdown)
	mgr.Sync(context.Background(), nil)

	sessMgr := session.NewManager(newMockStore(), bus, log)
	s := New(config.DefaultConfig(), bus, log, nil, sessMgr, mgr, nil, nil)

	result, err := s.handleListModels(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	models, ok := result.(modelListResult)
	if !ok {
		t.Fatalf("result = %T, want modelListResult", result)
	}
	if len(models.Models) != 1 || models.Models[0].Provider != providerName || models.Models[0].Model.ID != "team-model" {
		t.Fatalf("models = %+v", models.Models)
	}
}
