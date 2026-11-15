package pegglogin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

const DefaultsURL = "https://cdn.pegg.dev/defaults.json"

const defaultsFetchTimeout = 5 * time.Second

const defaultsMaxBytes = 1 << 20

type AgentDefaults struct {
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	FallbackProvider string `json:"fallback_provider,omitempty"`
	Fallback         string `json:"fallback,omitempty"`
}

type RouterAgentDefaults struct {
	Default    []string            `json:"default,omitempty"`
	Difficulty map[string][]string `json:"difficulty,omitempty"`
	Images     []string            `json:"images,omitempty"`
}

type SmartRouterDefaults struct {
	Enabled       bool                           `json:"enabled"`
	Provider      string                         `json:"provider,omitempty"`
	DecisionModel string                         `json:"decision_model,omitempty"`
	Agents        map[string]RouterAgentDefaults `json:"agents,omitempty"`
}

type STTDefaults struct {
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	StreamingModel string `json:"streaming_model,omitempty"`
}

type JudgeDefaults struct {
	Provider  string   `json:"provider,omitempty"`
	Model     string   `json:"model,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
}

type MemoryDefaults struct {
	Enabled   *bool            `json:"enabled,omitempty"`
	Gate      string           `json:"gate,omitempty"`
	Threshold *float64         `json:"threshold,omitempty"`
	Observer  *config.ModelRef `json:"observer,omitempty"`
	Relevance *config.ModelRef `json:"relevance,omitempty"`
}

type CompactionDefaults struct {
	SummarizerProvider string `json:"summarizer_provider,omitempty"`
	SummarizerModel    string `json:"summarizer_model,omitempty"`
}

type Defaults struct {
	Version     int                      `json:"version"`
	UpdatedAt   string                   `json:"updated_at,omitempty"`
	Agents      map[string]AgentDefaults `json:"agents,omitempty"`
	SmartRouter *SmartRouterDefaults     `json:"smart_router,omitempty"`
	STT         *STTDefaults             `json:"stt,omitempty"`
	Judge       *JudgeDefaults           `json:"judge,omitempty"`
	Memory      *MemoryDefaults          `json:"memory,omitempty"`
	Compaction  *CompactionDefaults      `json:"compaction,omitempty"`
}

type Report struct {
	Changes []string
	Skipped []string
}

func (r Report) Empty() bool {
	return len(r.Changes) == 0 && len(r.Skipped) == 0
}

func DefaultsEndpoint(flagURL string) string {
	if v := strings.TrimSpace(flagURL); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("PEGG_DEFAULTS_URL")); v != "" {
		return v
	}
	return DefaultsURL
}

func FetchDefaults(ctx context.Context, url string, client *http.Client) (*Defaults, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		url = DefaultsURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, defaultsFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: build defaults request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: fetch defaults: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, defaultsMaxBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pegglogin: defaults request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var d Defaults
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("pegglogin: decode defaults: %w", err)
	}
	if d.Version != 1 {
		return nil, fmt.Errorf("pegglogin: unsupported defaults version %d (expected 1)", d.Version)
	}
	return &d, nil
}

func ApplyDefaults(cfg *config.Config, d *Defaults, provider string) Report {
	var report Report
	if cfg == nil || d == nil {
		return report
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = ProviderName
	}
	available := make(map[string]bool, len(cfg.Providers))
	for _, p := range cfg.Providers {
		available[p.Provider] = true
	}
	resolve := func(explicit string) string {
		if v := strings.TrimSpace(explicit); v != "" {
			return v
		}
		return provider
	}
	unavailable := func(name string) bool {
		return name != "" && !available[name]
	}

	if len(d.Agents) > 0 {
		allowed := map[string]bool{"orchestrator": true, "explorer": true, "developer": true}
		names := make([]string, 0, len(d.Agents))
		for name := range d.Agents {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			agent := d.Agents[name]
			if !allowed[name] {
				report.Skipped = append(report.Skipped, fmt.Sprintf("agents.%s: unsupported agent", name))
				continue
			}
			if agent.Model == "" && agent.Fallback == "" {
				continue
			}

			agentProvider := resolve(agent.Provider)
			if agent.Model != "" && unavailable(agentProvider) {
				report.Skipped = append(report.Skipped, fmt.Sprintf("agents.%s: provider %q is not configured", name, agentProvider))
				continue
			}

			fallbackProvider := resolve(agent.FallbackProvider)
			if strings.TrimSpace(agent.FallbackProvider) == "" {
				fallbackProvider = agentProvider
			}
			fallbackModel := agent.Fallback
			if fallbackModel != "" && unavailable(fallbackProvider) {
				report.Skipped = append(report.Skipped, fmt.Sprintf("agents.%s.fallback: provider %q is not configured", name, fallbackProvider))
				fallbackModel = ""
				fallbackProvider = ""
			}

			if cfg.AgentDefaults == nil {
				cfg.AgentDefaults = make(map[string]config.AgentModelDefault)
			}
			cfg.AgentDefaults[name] = config.AgentModelDefault{
				Provider:         agentProvider,
				Model:            agent.Model,
				FallbackProvider: fallbackProvider,
				FallbackModel:    fallbackModel,
			}
			if agent.Model != "" {
				report.Changes = append(report.Changes, fmt.Sprintf("%s: %s/%s", name, agentProvider, agent.Model))
			}
			if fallbackModel != "" {
				report.Changes = append(report.Changes, fmt.Sprintf("%s fallback: %s/%s", name, fallbackProvider, fallbackModel))
			}
		}
	}

	if d.SmartRouter != nil {
		resolved := resolve(d.SmartRouter.Provider)
		if unavailable(resolved) {
			report.Skipped = append(report.Skipped, fmt.Sprintf("smart_router: provider %q is not configured", resolved))
		} else {
			agents := make(map[string]config.RouterAgentConfig, len(d.SmartRouter.Agents))
			for name, agent := range d.SmartRouter.Agents {
				agents[name] = config.RouterAgentConfig{
					Default:    agent.Default,
					Difficulty: agent.Difficulty,
					Images:     agent.Images,
				}
			}
			cfg.SmartRouter = &config.SmartRouterConfig{
				Enabled:  d.SmartRouter.Enabled,
				Provider: resolved,
				Model:    d.SmartRouter.DecisionModel,
				Agents:   agents,
			}
			report.Changes = append(report.Changes, fmt.Sprintf("smart_router: %s/%s", resolved, d.SmartRouter.DecisionModel))
		}
	}

	if d.STT != nil && (d.STT.Model != "" || d.STT.StreamingModel != "") {
		resolved := resolve(d.STT.Provider)
		if unavailable(resolved) {
			report.Skipped = append(report.Skipped, fmt.Sprintf("stt: provider %q is not configured", resolved))
		} else {
			if cfg.STT == nil {
				cfg.STT = &config.STTConfig{}
			}
			if cfg.STT.MaxDurationSecs == 0 {
				cfg.STT.MaxDurationSecs = config.DefaultSTTMaxDurationSecs
			}
			models := make([]config.STTModel, 0, 2)
			if d.STT.Model != "" {
				models = append(models, config.STTModel{ID: d.STT.Model})
			}
			if d.STT.StreamingModel != "" {
				models = append(models, config.STTModel{ID: d.STT.StreamingModel})
			}
			cfg.STT.UpsertProvider(config.STTProviderConfig{
				Provider: resolved,
				Driver:   driverFor(cfg, resolved),
				BaseURL:  baseURLFor(cfg, resolved),
				APIKey:   apiKeyFor(cfg, resolved),
				Models:   models,
			})
			if d.STT.Model != "" {
				cfg.STT.Provider = resolved
				cfg.STT.Model = d.STT.Model
				cfg.STT.StreamingModel = d.STT.StreamingModel
			}
			report.Changes = append(report.Changes, fmt.Sprintf("stt: %s/%s", resolved, d.STT.Model))
		}
	}

	if j := d.Judge; j != nil && (j.Model != "" || j.Threshold != nil) {
		resolved := resolve(j.Provider)
		if unavailable(resolved) {
			report.Skipped = append(report.Skipped, fmt.Sprintf("judge: provider %q is not configured", resolved))
		} else {
			if cfg.Permission == nil {
				cfg.Permission = &config.PermissionConfig{Default: "semi-ask"}
			}
			cfg.Permission.JudgeProvider = resolved
			cfg.Permission.JudgeModel = j.Model
			if j.Threshold != nil {
				cfg.Permission.JudgeThreshold = j.Threshold
			}
			report.Changes = append(report.Changes, fmt.Sprintf("judge: %s/%s", resolved, j.Model))
		}
	}

	if m := d.Memory; m != nil &&
		(m.Enabled != nil || m.Gate != "" || m.Threshold != nil ||
			(m.Observer != nil && m.Observer.Model != "") ||
			(m.Relevance != nil && m.Relevance.Model != "")) {
		if cfg.Memory == nil {
			cfg.Memory = &config.MemoryConfig{}
		}
		if d.Memory.Enabled != nil {
			cfg.Memory.Enabled = *d.Memory.Enabled
			report.Changes = append(report.Changes, fmt.Sprintf("memory: enabled=%t", *d.Memory.Enabled))
		}
		if d.Memory.Gate != "" {
			cfg.Memory.Gate = d.Memory.Gate
			report.Changes = append(report.Changes, fmt.Sprintf("memory: gate=%s", d.Memory.Gate))
		}
		if d.Memory.Threshold != nil {
			cfg.Memory.Threshold = d.Memory.Threshold
			report.Changes = append(report.Changes, fmt.Sprintf("memory: threshold=%.2f", *d.Memory.Threshold))
		}
		if d.Memory.Observer != nil {
			obsProvider := resolve(d.Memory.Observer.Provider)
			if unavailable(obsProvider) {
				report.Skipped = append(report.Skipped, fmt.Sprintf("memory.observer: provider %q is not configured", obsProvider))
			} else {
				cfg.Memory.Observer = &config.ModelRef{Provider: obsProvider, Model: d.Memory.Observer.Model}
				report.Changes = append(report.Changes, fmt.Sprintf("memory.observer: %s/%s", obsProvider, d.Memory.Observer.Model))
			}
		}
		if d.Memory.Relevance != nil {
			relProvider := resolve(d.Memory.Relevance.Provider)
			if unavailable(relProvider) {
				report.Skipped = append(report.Skipped, fmt.Sprintf("memory.relevance: provider %q is not configured", relProvider))
			} else {
				cfg.Memory.Relevance = &config.ModelRef{Provider: relProvider, Model: d.Memory.Relevance.Model}
				report.Changes = append(report.Changes, fmt.Sprintf("memory.relevance: %s/%s", relProvider, d.Memory.Relevance.Model))
			}
		}
	}

	if d.Compaction != nil {
		resolved := resolve(d.Compaction.SummarizerProvider)
		if unavailable(resolved) {
			report.Skipped = append(report.Skipped, fmt.Sprintf("compaction: provider %q is not configured", resolved))
		} else {
			if cfg.Compaction == nil {
				cfg.Compaction = &config.CompactionConfig{}
			}
			cfg.Compaction.SummarizerProvider = resolved
			cfg.Compaction.SummarizerModel = d.Compaction.SummarizerModel
			if !containsString(cfg.Compaction.Strategy, "summarization") {
				cfg.Compaction.Strategy = append(cfg.Compaction.Strategy, "summarization")
			}
			report.Changes = append(report.Changes, fmt.Sprintf("compaction: %s/%s (summarization enabled)", resolved, d.Compaction.SummarizerModel))
		}
	}

	return report
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func providerConfigFor(cfg *config.Config, name string) (config.LLMConfig, bool) {
	if cfg == nil {
		return config.LLMConfig{}, false
	}
	for _, p := range cfg.Providers {
		if p.Provider == name {
			return p, true
		}
	}
	return config.LLMConfig{}, false
}

func driverFor(cfg *config.Config, name string) string {
	if p, ok := providerConfigFor(cfg, name); ok && p.Driver != "" {
		return p.Driver
	}
	return defaultDriver
}

func baseURLFor(cfg *config.Config, name string) string {
	if p, ok := providerConfigFor(cfg, name); ok {
		return p.BaseURL
	}
	return ""
}

func apiKeyFor(cfg *config.Config, name string) string {
	if p, ok := providerConfigFor(cfg, name); ok {
		return p.APIKey
	}
	return ""
}
