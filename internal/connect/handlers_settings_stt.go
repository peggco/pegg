package connect

import (
	"context"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

func (c *Connector) registerSTTSettings() {
	c.dispatch.register("settings.stt.get", ScopeSettingsRead, c.handleSTTGet)
	c.dispatch.register("settings.stt.update", ScopeSettingsWrite, c.handleSTTUpdate)
}

func (c *Connector) sttConfig() *config.STTConfig {
	if c.deps.Config == nil || c.deps.Config.STT == nil {
		return &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
	}
	return c.deps.Config.STT
}

func configuredSTTProviders(cfg *config.STTConfig) []string {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	for _, p := range cfg.Providers {
		if p.Provider == "" {
			continue
		}
		if _, ok := seen[p.Provider]; ok {
			continue
		}
		seen[p.Provider] = struct{}{}
		names = append(names, p.Provider)
	}
	if len(names) == 0 && cfg.Provider != "" && cfg.APIKey != "" {
		names = append(names, cfg.Provider)
	}
	return names
}

func sttModelsForConfig(cfg *config.STTConfig) []stt.Model {
	var models []stt.Model
	for _, name := range configuredSTTProviders(cfg) {
		models = append(models, stt.ModelsForConfig(name, cfg)...)
	}
	return models
}

func (c *Connector) handleSTTGet(_ context.Context, _ *Request) (any, error) {
	cfg := c.sttConfig()
	hasKey := false
	if entry, ok := cfg.ProviderConfig(cfg.Provider); ok {
		hasKey = entry.APIKey != ""
	}
	return map[string]any{
		"provider":          cfg.Provider,
		"model":             cfg.Model,
		"has_key":           hasKey,
		"providers":         configuredSTTProviders(cfg),
		"models":            sttModelsForConfig(cfg),
		"max_duration_secs": cfg.MaxDurationValue(),
	}, nil
}

type sttUpdateParams struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	APIKey   *string `json:"api_key"`
	BaseURL  *string `json:"base_url"`
}

func (c *Connector) handleSTTUpdate(_ context.Context, req *Request) (any, error) {
	var p sttUpdateParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	cfg := *c.sttConfig()
	if p.Provider != "" {
		cfg.Provider = p.Provider
	}
	if p.Model != "" {
		cfg.Model = p.Model
	}
	if p.APIKey != nil && *p.APIKey != "" {
		if err := c.requireConfirm("write STT API key", cfg.Provider); err != nil {
			return nil, err
		}
		entry, _ := cfg.ProviderConfig(cfg.Provider)
		entry.Provider = cfg.Provider
		entry.APIKey = *p.APIKey
		if p.BaseURL != nil {
			entry.BaseURL = *p.BaseURL
		}
		cfg.UpsertProvider(entry)
	}
	if p.BaseURL != nil && *p.BaseURL != "" {
		entry, _ := cfg.ProviderConfig(cfg.Provider)
		entry.Provider = cfg.Provider
		entry.BaseURL = *p.BaseURL
		cfg.UpsertProvider(entry)
	}
	if err := config.UpsertSTT(&cfg); err != nil {
		return nil, &FrameError{Code: ErrCodeInternal, Message: "save stt settings: " + err.Error()}
	}
	if err := c.reloadConfig(); err != nil {
		return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
	}
	c.emit(eventSettings, "", "", map[string]any{"group": "voice"})
	return map[string]any{
		"provider": cfg.Provider,
		"model":    cfg.Model,
		"has_key":  true,
	}, nil
}
