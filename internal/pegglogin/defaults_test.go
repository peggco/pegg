package pegglogin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }

func stringDefault(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.Providers = []config.LLMConfig{
		{
			Provider: ProviderName,
			Driver:   "openai",
			BaseURL:  DefaultBaseURL,
			APIKey:   "vsk_live_test",
		},
	}
}

func TestDefaultsEndpointPrecedence(t *testing.T) {
	t.Setenv("PEGG_DEFAULTS_URL", "https://env.example/defaults.json")
	if got := DefaultsEndpoint("https://flag.example/defaults.json"); got != "https://flag.example/defaults.json" {
		t.Fatalf("flag endpoint = %q", got)
	}
	if got := DefaultsEndpoint("  "); got != "https://env.example/defaults.json" {
		t.Fatalf("env endpoint = %q", got)
	}
	t.Setenv("PEGG_DEFAULTS_URL", "")
	if got := DefaultsEndpoint(""); got != DefaultsURL {
		t.Fatalf("default endpoint = %q, want %q", got, DefaultsURL)
	}
}

func TestFetchDefaultsHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/defaults.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"version":1,"updated_at":"2026-01-01","agents":{"orchestrator":{"model":"deepseek-v4-flash","fallback":"deepseek-v4-lite"}},"judge":{"provider":"openrouter","model":"typesafe/jev-1.13","threshold":0.7}}`)
	}))
	defer srv.Close()

	d, err := FetchDefaults(context.Background(), srv.URL+"/defaults.json", srv.Client())
	if err != nil {
		t.Fatalf("fetch defaults: %v", err)
	}
	if d.Version != 1 || d.UpdatedAt != "2026-01-01" {
		t.Fatalf("unexpected envelope: %#v", d)
	}
	if d.Agents["orchestrator"].Model != "deepseek-v4-flash" {
		t.Fatalf("unexpected agent defaults: %#v", d.Agents)
	}
	if d.Judge == nil || d.Judge.Model != "typesafe/jev-1.13" || d.Judge.Threshold == nil || *d.Judge.Threshold != 0.7 {
		t.Fatalf("unexpected judge defaults: %#v", d.Judge)
	}
}

func TestFetchDefaultsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer srv.Close()

	_, err := FetchDefaults(context.Background(), srv.URL, srv.Client())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v, want HTTP 500", err)
	}
}

func TestFetchDefaultsMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"version":`)
	}))
	defer srv.Close()

	_, err := FetchDefaults(context.Background(), srv.URL, srv.Client())
	if err == nil || !strings.Contains(err.Error(), "decode defaults") {
		t.Fatalf("err = %v, want decode error", err)
	}
}

func TestFetchDefaultsWrongVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"version":2}`)
	}))
	defer srv.Close()

	_, err := FetchDefaults(context.Background(), srv.URL, srv.Client())
	if err == nil || !strings.Contains(err.Error(), "unsupported defaults version 2") {
		t.Fatalf("err = %v, want version error", err)
	}
}

func TestApplyDefaultsHappyPath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{
		{Provider: ProviderName, Driver: "openai", BaseURL: DefaultBaseURL, APIKey: "key"},
		{Provider: "openrouter", Driver: "openai", BaseURL: "https://openrouter.ai/api/v1", APIKey: "or-key"},
	}
	cfg.Compaction.Strategy = []string{"sliding-window"}

	d := &Defaults{
		Version: 1,
		Agents: map[string]AgentDefaults{
			"orchestrator": {Model: "deepseek-v4-flash", Fallback: "deepseek-v4-lite"},
			"unknown":      {Model: "nope"},
		},
		SmartRouter: &SmartRouterDefaults{
			Enabled:       true,
			DecisionModel: "typesafe/jev-1.13",
			Agents: map[string]RouterAgentDefaults{
				"orchestrator": {Default: []string{"deepseek-v4-flash"}},
			},
		},
		STT: &STTDefaults{Model: "whisper-1", StreamingModel: "whisper-stream"},
		Judge: &JudgeDefaults{
			Provider:  "openrouter",
			Model:     "typesafe/jev-1.13",
			Threshold: floatPtr(0.7),
		},
		Memory: &MemoryDefaults{
			Enabled:   boolPtr(false),
			Gate:      "auto",
			Threshold: floatPtr(0.5),
			Observer:  &config.ModelRef{Model: "observer-model"},
			Relevance: &config.ModelRef{Provider: "openrouter", Model: "relevance-model"},
		},
		Compaction: &CompactionDefaults{SummarizerModel: "deepseek-v4-flash"},
	}

	report := ApplyDefaults(cfg, d, ProviderName)

	agent := cfg.AgentDefaults["orchestrator"]
	if agent.Provider != ProviderName || agent.Model != "deepseek-v4-flash" || agent.FallbackProvider != ProviderName || agent.FallbackModel != "deepseek-v4-lite" {
		t.Fatalf("unexpected agent defaults: %#v", agent)
	}
	if _, ok := cfg.AgentDefaults["unknown"]; ok {
		t.Fatal("unsupported agent should not be applied")
	}
	if !hasPrefix(report.Skipped, "agents.unknown:") {
		t.Fatalf("missing unsupported agent skip: %#v", report.Skipped)
	}
	if cfg.SmartRouter == nil || !cfg.SmartRouter.Enabled || cfg.SmartRouter.Provider != ProviderName || cfg.SmartRouter.Model != "typesafe/jev-1.13" {
		t.Fatalf("unexpected smart router: %#v", cfg.SmartRouter)
	}
	if cfg.SmartRouter.Agents["orchestrator"].Default[0] != "deepseek-v4-flash" {
		t.Fatalf("unexpected router agents: %#v", cfg.SmartRouter.Agents)
	}
	if cfg.STT == nil || cfg.STT.Provider != ProviderName || cfg.STT.Model != "whisper-1" || cfg.STT.StreamingModel != "whisper-stream" {
		t.Fatalf("unexpected stt: %#v", cfg.STT)
	}
	if cfg.STT.MaxDurationSecs != config.DefaultSTTMaxDurationSecs {
		t.Fatalf("max duration = %d", cfg.STT.MaxDurationSecs)
	}
	sttProvider, ok := cfg.STT.ProviderConfig(ProviderName)
	if !ok {
		t.Fatal("stt provider not upserted")
	}
	if sttProvider.Driver != "openai" || sttProvider.BaseURL != DefaultBaseURL || sttProvider.APIKey != "key" {
		t.Fatalf("unexpected stt provider: %#v", sttProvider)
	}
	if len(sttProvider.Models) != 2 {
		t.Fatalf("stt models = %#v", sttProvider.Models)
	}
	if cfg.Permission.JudgeProvider != "openrouter" || cfg.Permission.JudgeModel != "typesafe/jev-1.13" {
		t.Fatalf("unexpected judge: %#v", cfg.Permission)
	}
	if cfg.Permission.JudgeThreshold == nil || *cfg.Permission.JudgeThreshold != 0.7 {
		t.Fatalf("unexpected judge threshold: %#v", cfg.Permission.JudgeThreshold)
	}
	if cfg.Memory.Enabled || cfg.Memory.Gate != "auto" || cfg.Memory.Threshold == nil || *cfg.Memory.Threshold != 0.5 {
		t.Fatalf("unexpected memory: %#v", cfg.Memory)
	}
	if cfg.Memory.Observer == nil || cfg.Memory.Observer.Provider != ProviderName || cfg.Memory.Observer.Model != "observer-model" {
		t.Fatalf("unexpected observer: %#v", cfg.Memory.Observer)
	}
	if cfg.Memory.Relevance == nil || cfg.Memory.Relevance.Provider != "openrouter" {
		t.Fatalf("unexpected relevance: %#v", cfg.Memory.Relevance)
	}
	if cfg.Compaction.SummarizerProvider != ProviderName || cfg.Compaction.SummarizerModel != "deepseek-v4-flash" {
		t.Fatalf("unexpected compaction: %#v", cfg.Compaction)
	}
	if !containsString(cfg.Compaction.Strategy, "summarization") {
		t.Fatalf("strategy = %#v, want summarization", cfg.Compaction.Strategy)
	}
	if report.Empty() || len(report.Changes) == 0 {
		t.Fatalf("expected changes, got %#v", report)
	}
}

func TestApplyDefaultsMissingProviderSkips(t *testing.T) {
	cfg := config.DefaultConfig()
	stringDefault(t, cfg)

	d := &Defaults{
		Version: 1,
		Agents:  map[string]AgentDefaults{"orchestrator": {Model: "deepseek-v4-flash"}},
		SmartRouter: &SmartRouterDefaults{
			Enabled:  true,
			Provider: "openrouter",
		},
		Judge: &JudgeDefaults{Provider: "openrouter", Model: "typesafe/jev-1.13"},
		Memory: &MemoryDefaults{
			Relevance: &config.ModelRef{Provider: "openrouter", Model: "relevance-model"},
		},
		Compaction: &CompactionDefaults{SummarizerProvider: "openrouter", SummarizerModel: "m"},
	}

	report := ApplyDefaults(cfg, d, ProviderName)

	if cfg.AgentDefaults["orchestrator"].Model != "deepseek-v4-flash" {
		t.Fatalf("agents should still apply: %#v", cfg.AgentDefaults)
	}
	if cfg.SmartRouter != nil && cfg.SmartRouter.Provider == "openrouter" {
		t.Fatal("unavailable smart router provider should be skipped")
	}
	if cfg.Permission != nil && cfg.Permission.JudgeProvider == "openrouter" {
		t.Fatal("unavailable judge provider should be skipped")
	}
	if cfg.Memory != nil && cfg.Memory.Relevance != nil {
		t.Fatal("unavailable memory relevance provider should be skipped")
	}
	for _, key := range []string{"smart_router:", "judge:", "memory.relevance:", "compaction:"} {
		if !hasPrefix(report.Skipped, key) {
			t.Fatalf("missing skip %q in %#v", key, report.Skipped)
		}
	}
	for _, skipped := range report.Skipped {
		if !strings.Contains(skipped, `provider "openrouter" is not configured`) {
			t.Fatalf("unexpected skip text: %q", skipped)
		}
	}
}

func TestApplyDefaultsEmptyProviderUsesProviderName(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{{Provider: ProviderName, Driver: "openai"}}
	cfg.SmartRouter = nil
	cfg.Permission = nil
	cfg.Memory = nil
	cfg.Compaction = nil
	cfg.STT = nil
	cfg.AgentDefaults = nil

	d := &Defaults{
		Version: 1,
		Agents:  map[string]AgentDefaults{"developer": {Model: "m"}},
		SmartRouter: &SmartRouterDefaults{
			Enabled:       true,
			DecisionModel: "d",
		},
		STT:        &STTDefaults{Model: "s"},
		Judge:      &JudgeDefaults{Model: "j"},
		Memory:     &MemoryDefaults{Relevance: &config.ModelRef{Model: "r"}},
		Compaction: &CompactionDefaults{SummarizerModel: "c"},
	}

	report := ApplyDefaults(cfg, d, "")

	if cfg.AgentDefaults["developer"].Provider != ProviderName {
		t.Fatalf("agent provider = %q", cfg.AgentDefaults["developer"].Provider)
	}
	if cfg.SmartRouter == nil || cfg.SmartRouter.Provider != ProviderName {
		t.Fatalf("smart router = %#v", cfg.SmartRouter)
	}
	if cfg.STT == nil || cfg.STT.Provider != ProviderName {
		t.Fatalf("stt = %#v", cfg.STT)
	}
	if cfg.Permission == nil || cfg.Permission.JudgeProvider != ProviderName {
		t.Fatalf("judge = %#v", cfg.Permission)
	}
	if cfg.Memory == nil || cfg.Memory.Relevance == nil || cfg.Memory.Relevance.Provider != ProviderName {
		t.Fatalf("memory = %#v", cfg.Memory)
	}
	if cfg.Compaction == nil || cfg.Compaction.SummarizerProvider != ProviderName {
		t.Fatalf("compaction = %#v", cfg.Compaction)
	}
	if len(report.Skipped) != 0 {
		t.Fatalf("unexpected skips: %#v", report.Skipped)
	}
}

func TestReportEmpty(t *testing.T) {
	if !(Report{}).Empty() {
		t.Fatal("zero report should be empty")
	}
	if (Report{Changes: []string{"x"}}).Empty() {
		t.Fatal("report with changes should not be empty")
	}
	if (Report{Skipped: []string{"x"}}).Empty() {
		t.Fatal("report with skips should not be empty")
	}
}

func hasPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func TestApplyDefaultsAgentProviders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{
		{Provider: ProviderName, Driver: "openai", BaseURL: DefaultBaseURL, APIKey: "key"},
		{Provider: "openrouter", Driver: "openai", BaseURL: "https://openrouter.ai/api/v1", APIKey: "or-key"},
	}

	d := &Defaults{
		Version: 1,
		Agents: map[string]AgentDefaults{
			"orchestrator": {
				Provider:         "openrouter",
				Model:            "router-model",
				FallbackProvider: ProviderName,
				Fallback:         "forever-model",
			},
			"developer": {Provider: "missing", Model: "nope"},
			"explorer": {
				Provider:         "openrouter",
				Model:            "explorer-model",
				FallbackProvider: "missing",
				Fallback:         "nope",
			},
		},
	}

	report := ApplyDefaults(cfg, d, ProviderName)

	orchestrator := cfg.AgentDefaults["orchestrator"]
	if orchestrator.Provider != "openrouter" || orchestrator.Model != "router-model" ||
		orchestrator.FallbackProvider != ProviderName || orchestrator.FallbackModel != "forever-model" {
		t.Fatalf("unexpected orchestrator defaults: %#v", orchestrator)
	}
	if _, ok := cfg.AgentDefaults["developer"]; ok {
		t.Fatal("agent with missing provider should not be applied")
	}
	if !hasPrefix(report.Skipped, "agents.developer:") {
		t.Fatalf("missing developer skip: %#v", report.Skipped)
	}
	explorer := cfg.AgentDefaults["explorer"]
	if explorer.Provider != "openrouter" || explorer.Model != "explorer-model" {
		t.Fatalf("unexpected explorer defaults: %#v", explorer)
	}
	if explorer.FallbackProvider != "" || explorer.FallbackModel != "" {
		t.Fatalf("unavailable fallback should be dropped: %#v", explorer)
	}
	if !hasPrefix(report.Skipped, "agents.explorer.fallback:") {
		t.Fatalf("missing fallback skip: %#v", report.Skipped)
	}
}
