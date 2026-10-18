package together

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	driver "github.com/peggco/pegg/internal/stt/drivers/openai"
)

const ProviderName = "together"

func init() {
	stt.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
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
		baseURL = "https://api.together.xyz/v1"
	}
	return driver.New(driver.ServiceConfig{
		Name:    ProviderName,
		BaseURL: baseURL,
		APIKey:  cfg.APIKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}), nil
}
