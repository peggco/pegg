package team

import (
	"errors"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	driver "github.com/peggco/pegg/internal/stt/drivers/openai"
)

const ProviderName = "pegg-team"

func init() {
	stt.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
	baseURL := cfg.BaseURL
	apiKey := cfg.APIKey
	if settings != nil {
		if entry, ok := settings.ProviderConfig(ProviderName); ok {
			if entry.BaseURL != "" {
				baseURL = entry.BaseURL
			}
			if entry.APIKey != "" {
				apiKey = entry.APIKey
			}
		}
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("stt: pegg-team provider is not configured; run `pegg team`")
	}
	return driver.New(driver.ServiceConfig{
		Name:    ProviderName,
		BaseURL: baseURL,
		APIKey:  apiKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}), nil
}
