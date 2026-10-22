package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
)

func TestApplyTeamConfigSystemFirstOnly(t *testing.T) {
	peggCfg := config.DefaultConfig()
	enabled := true
	cfg := &teamConfig{
		Team: struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: 7, Name: "Acme"},
		Provider: teamProviderConfig{
			Name:            "pegg-team Acme",
			Driver:          "openai",
			BaseURL:         "https://team.example.com/v1",
			APIKey:          "vsk_live_test",
			Timeout:         120,
			SystemFirstOnly: &enabled,
		},
	}

	applyTeamConfig(peggCfg, cfg)

	for i := range peggCfg.Providers {
		if peggCfg.Providers[i].Provider == "pegg-team Acme" {
			if !peggCfg.Providers[i].SystemFirstOnlyValue() {
				t.Fatal("system_first_only was not applied to the team provider")
			}
			return
		}
	}
	t.Fatal("team provider was not added")
}

func TestApplyTeamConfigModels(t *testing.T) {
	peggCfg := config.DefaultConfig()
	cfg := &teamConfig{
		Team: struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: 7, Name: "Acme"},
		Provider: teamProviderConfig{
			Name:    "pegg-team Acme",
			Driver:  "openai",
			BaseURL: "https://team.example.com/v1",
			APIKey:  "vsk_live_test",
		},
		Models: []teamModelConfig{
			{
				ID:     "gpt-4.1",
				Name:   "GPT-4.1",
				Kind:   "llm",
				Config: []byte(`{"max_input_tokens":1047576,"modalities":{"input":["text"]},"tool_call":true}`),
			},
			{ID: "plain", Name: "Plain"},
		},
	}

	applyTeamConfig(peggCfg, cfg)

	var provider *config.LLMConfig
	for i := range peggCfg.Providers {
		if peggCfg.Providers[i].Provider == "pegg-team Acme" {
			provider = &peggCfg.Providers[i]
		}
	}
	if provider == nil {
		t.Fatal("team provider was not added")
	}
	if len(provider.Models) != 2 {
		t.Fatalf("models = %+v", provider.Models)
	}
	first := provider.Models[0]
	if first.ID != "gpt-4.1" || first.Name != "GPT-4.1" {
		t.Fatalf("model = %+v", first)
	}
	if first.Config == nil || first.Config.MaxInputTokens != 1047576 {
		t.Fatalf("config = %+v", first.Config)
	}
	if first.Config.Modalities == nil || len(first.Config.Modalities.Input) != 1 {
		t.Fatalf("modalities = %+v", first.Config.Modalities)
	}
	if !first.Config.ToolCall {
		t.Fatal("tool_call must be carried over")
	}
	if provider.Models[1].Config != nil {
		t.Fatalf("plain model must not have a config: %+v", provider.Models[1].Config)
	}
}

func TestApplyTeamConfig(t *testing.T) {
	peggCfg := config.DefaultConfig()
	cfg := &teamConfig{
		Team: struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: 7, Name: "Acme"},
		Provider: teamProviderConfig{
			Name:    "pegg-team Acme",
			Driver:  "openai",
			BaseURL: "https://team.example.com/v1",
			APIKey:  "vsk_live_test",
			Timeout: 120,
		},
		Defaults: teamDefaults{
			Agents: map[string]teamAgentDefault{
				"orchestrator": {Model: "gpt-4.1", Fallback: "gpt-4.1-mini"},
			},
			SmartRouter: &teamSmartRouter{
				Enabled:       true,
				DecisionModel: "gpt-4.1",
				Agents: map[string]teamRouterAgent{
					"orchestrator": {
						Default: []string{"gpt-4.1"},
						Difficulty: map[string][]string{
							"complex": {"gpt-4.1"},
						},
					},
				},
			},
			STT: &teamSTT{
				Model:          "whisper-large-v3",
				StreamingModel: "whisper-large-v3",
			},
		},
	}

	applyTeamConfig(peggCfg, cfg)

	var provider *config.LLMConfig
	for i := range peggCfg.Providers {
		if peggCfg.Providers[i].Provider == "pegg-team Acme" {
			provider = &peggCfg.Providers[i]
		}
	}
	if provider == nil {
		t.Fatal("team provider was not added")
	}
	if provider.Driver != "openai" || provider.APIKey != "vsk_live_test" {
		t.Fatalf("provider = %#v", provider)
	}
	if got := peggCfg.AgentDefaults["orchestrator"]; got.Model != "gpt-4.1" || got.Provider != "pegg-team Acme" {
		t.Fatalf("agent default = %#v", got)
	}
	if peggCfg.SmartRouter == nil || !peggCfg.SmartRouter.Enabled {
		t.Fatal("smart router was not enabled")
	}
	if peggCfg.SmartRouter.Provider != "pegg-team Acme" || peggCfg.SmartRouter.Model != "gpt-4.1" {
		t.Fatalf("smart router = %#v", peggCfg.SmartRouter)
	}
	if peggCfg.STT == nil || peggCfg.STT.Provider != "pegg-team" || peggCfg.STT.Model != "whisper-large-v3" {
		t.Fatalf("stt = %#v", peggCfg.STT)
	}
	if _, ok := peggCfg.STT.ProviderConfig("pegg-team"); !ok {
		t.Fatal("pegg-team stt provider was not stored")
	}
}

func TestTeamExchangeURL(t *testing.T) {
	cases := map[string]string{
		"https://api.pegg.dev":     "https://api.pegg.dev/api/team-ai/cli/exchange",
		"https://api.pegg.dev/api": "https://api.pegg.dev/api/team-ai/cli/exchange",
		"https://pegg.dev":         "https://pegg.dev/api/team-ai/cli/exchange",
		"http://localhost:3000/":   "http://localhost:3000/api/team-ai/cli/exchange",
	}
	for input, want := range cases {
		if got := teamExchangeURL(input); got != want {
			t.Fatalf("teamExchangeURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsTeamProvider(t *testing.T) {
	for _, name := range []string{"pegg-team", "pegg-team Acme"} {
		if !isTeamProvider(name) {
			t.Fatalf("%q should be a team provider", name)
		}
	}
	for _, name := range []string{"openai", "team-pegg", ""} {
		if isTeamProvider(name) {
			t.Fatalf("%q should not be a team provider", name)
		}
	}
}

func TestVerifyTeamProviderRetries(t *testing.T) {
	oldAttempts, oldDelay := teamVerifyAttempts, teamVerifyDelay
	teamVerifyAttempts, teamVerifyDelay = 3, time.Millisecond
	t.Cleanup(func() {
		teamVerifyAttempts, teamVerifyDelay = oldAttempts, oldDelay
	})

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"not synced"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := verifyTeamProvider(server.URL, "vsk_test"); err != nil {
		t.Fatalf("verifyTeamProvider: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestVerifyTeamProviderIncludesResponseBody(t *testing.T) {
	oldAttempts, oldDelay := teamVerifyAttempts, teamVerifyDelay
	teamVerifyAttempts, teamVerifyDelay = 1, 0
	t.Cleanup(func() {
		teamVerifyAttempts, teamVerifyDelay = oldAttempts, oldDelay
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid or missing api key"}}`))
	}))
	defer server.Close()

	err := verifyTeamProvider(server.URL, "vsk_test")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "HTTP 401") ||
		!strings.Contains(err.Error(), "invalid or missing api key") {
		t.Fatalf("error = %q, want status and response body", err)
	}
}
