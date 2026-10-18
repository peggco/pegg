package openai

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	driver "github.com/peggco/pegg/internal/stt/drivers/openai"
)

const ProviderName = "openai"

func init() {
	stt.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
	return newService(ProviderName, "https://api.openai.com/v1", cfg, settings), nil
}

func newService(name, defaultURL string, cfg config.LLMConfig, settings *config.STTConfig) stt.Provider {
	baseURL := ""
	if settings != nil {
		if entry, ok := settings.ProviderConfig(ProviderName); ok {
			baseURL = entry.BaseURL
		}
	}
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}
	if baseURL == "" {
		baseURL = defaultURL
	}
	return driver.New(driver.ServiceConfig{
		Name:    name,
		BaseURL: baseURL,
		APIKey:  cfg.APIKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	})
}
