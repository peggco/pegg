package memory

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	decisionapi "github.com/peggco/pegg/internal/decision"
	"github.com/peggco/pegg/internal/llm"
)

type resolveLLMProvider struct {
	name   string
	models []llm.Model
}

func (p *resolveLLMProvider) Name() string { return p.name }

func (p *resolveLLMProvider) Chat(context.Context, *llm.Request) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (p *resolveLLMProvider) ChatStream(context.Context, *llm.Request, llm.StreamHandler) error {
	return nil
}

func (p *resolveLLMProvider) ListModels(context.Context) ([]llm.Model, error) {
	return p.models, nil
}

func newResolveLLM(t *testing.T, providers map[string][]string) *llm.Manager {
	t.Helper()
	bus := event.New()
	mgr := llm.NewManager(bus, testLogger(), testCache(t))
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Shutdown)

	var cfgs []config.LLMConfig
	for name, ids := range providers {
		models := make([]llm.Model, 0, len(ids))
		for _, id := range ids {
			models = append(models, llm.Model{ID: id})
		}
		llm.RegisterProvider(name, func(cfg config.LLMConfig) (llm.Provider, error) {
			return &resolveLLMProvider{name: cfg.Provider, models: models}, nil
		})
		cfgs = append(cfgs, config.LLMConfig{Provider: name})
	}
	mgr.Sync(context.Background(), cfgs)
	return mgr
}

func newResolveDecision(t *testing.T, registered []string, loaded []string) *decisionapi.Manager {
	t.Helper()
	bus := event.New()
	mgr := decisionapi.NewManager(bus, testLogger())
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Shutdown)

	for _, name := range registered {
		name := name
		decisionapi.RegisterProvider(name, func(cfg config.LLMConfig) (decisionapi.Provider, error) {
			return &mockDecision{name: cfg.Provider, answers: map[string]float64{}}, nil
		})
	}
	providers := make([]config.LLMConfig, 0, len(loaded))
	for _, name := range loaded {
		providers = append(providers, config.LLMConfig{Provider: name, APIKey: "k"})
	}
	bus.Publish(event.TopicAppMounted, &config.Config{Providers: providers})
	return mgr
}

func TestObserverModelPrefersConfiguredObserver(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	llmMgr := newResolveLLM(t, map[string][]string{"mem-res-a": {"m1", "m2"}})
	cfg := defaultCfg()
	cfg.Observer = &config.ModelRef{Provider: "mem-res-a", Model: "m2"}
	m := newManager(Deps{Config: cfg, LLM: llmMgr}, t.TempDir())

	prov, model, err := m.observerModel()
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "mem-res-a" || model.ID != "m2" {
		t.Fatalf("observer = %s/%s, want mem-res-a/m2", prov.Name(), model.ID)
	}
}

func TestObserverModelInfersProviderFromModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	llmMgr := newResolveLLM(t, map[string][]string{
		"mem-res-b": {"b1"},
		"mem-res-c": {"c1", "c2"},
	})
	cfg := defaultCfg()
	cfg.Observer = &config.ModelRef{Model: "c2"}
	m := newManager(Deps{Config: cfg, LLM: llmMgr}, t.TempDir())

	prov, model, err := m.observerModel()
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "mem-res-c" || model.ID != "c2" {
		t.Fatalf("observer = %s/%s, want mem-res-c/c2", prov.Name(), model.ID)
	}
}

func TestObserverModelLegacyFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	llmMgr := newResolveLLM(t, map[string][]string{"mem-res-d": {"d1"}})
	cfg := defaultCfg()
	cfg.Provider = "mem-res-d"
	cfg.Model = "d1"
	m := newManager(Deps{Config: cfg, LLM: llmMgr}, t.TempDir())

	prov, model, err := m.observerModel()
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "mem-res-d" || model.ID != "d1" {
		t.Fatalf("observer = %s/%s, want mem-res-d/d1", prov.Name(), model.ID)
	}
}

func TestDeciderPrefersConfiguredRelevance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	decMgr := newResolveDecision(t,
		[]string{"mem-res-dec-a", "mem-res-dec-b"},
		[]string{"mem-res-dec-a", "mem-res-dec-b"},
	)
	cfg := defaultCfg()
	cfg.Relevance = &config.ModelRef{Provider: "mem-res-dec-b", Model: "judge"}
	m := newManager(Deps{Config: cfg, Decision: decMgr}, t.TempDir())

	prov, model, ok := m.decider()
	if !ok {
		t.Fatal("decider unavailable")
	}
	if prov.Name() != "mem-res-dec-b" || model != "judge" {
		t.Fatalf("decider = %s/%s, want mem-res-dec-b/judge", prov.Name(), model)
	}
}

func TestDeciderLegacyFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	decMgr := newResolveDecision(t,
		[]string{"mem-res-dec-c"},
		[]string{"mem-res-dec-c"},
	)
	cfg := defaultCfg()
	cfg.Provider = "mem-res-dec-c"
	cfg.Model = "legacy"
	m := newManager(Deps{Config: cfg, Decision: decMgr}, t.TempDir())

	prov, model, ok := m.decider()
	if !ok {
		t.Fatal("decider unavailable")
	}
	if prov.Name() != "mem-res-dec-c" || model != "legacy" {
		t.Fatalf("decider = %s/%s, want mem-res-dec-c/legacy", prov.Name(), model)
	}
}

func TestDeciderUnavailableRelevanceFallsBackToPreferred(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	decMgr := newResolveDecision(t,
		[]string{"mem-res-dec-d", "mem-res-dec-missing"},
		[]string{"mem-res-dec-d"},
	)
	cfg := defaultCfg()
	cfg.Relevance = &config.ModelRef{Provider: "mem-res-dec-missing", Model: "judge"}
	m := newManager(Deps{Config: cfg, Decision: decMgr}, t.TempDir())

	prov, model, ok := m.decider()
	if !ok {
		t.Fatal("decider unavailable")
	}
	if prov.Name() != "mem-res-dec-d" || model != "" {
		t.Fatalf("decider = %s/%s, want mem-res-dec-d/\"\"", prov.Name(), model)
	}
}

func TestResolveObserverAndRelevanceTogether(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	llmMgr := newResolveLLM(t, map[string][]string{"mem-res-e": {"e1", "e2"}})
	decMgr := newResolveDecision(t,
		[]string{"mem-res-dec-e"},
		[]string{"mem-res-dec-e"},
	)
	cfg := defaultCfg()
	cfg.Provider = "mem-res-legacy"
	cfg.Model = "legacy-model"
	cfg.Observer = &config.ModelRef{Provider: "mem-res-e", Model: "e2"}
	cfg.Relevance = &config.ModelRef{Provider: "mem-res-dec-e", Model: "judge"}
	m := newManager(Deps{Config: cfg, LLM: llmMgr, Decision: decMgr}, t.TempDir())

	prov, model, err := m.observerModel()
	if err != nil {
		t.Fatal(err)
	}
	if prov.Name() != "mem-res-e" || model.ID != "e2" {
		t.Fatalf("observer = %s/%s, want mem-res-e/e2", prov.Name(), model.ID)
	}

	dprov, dmodel, ok := m.decider()
	if !ok {
		t.Fatal("decider unavailable")
	}
	if dprov.Name() != "mem-res-dec-e" || dmodel != "judge" {
		t.Fatalf("decider = %s/%s, want mem-res-dec-e/judge", dprov.Name(), dmodel)
	}
}
