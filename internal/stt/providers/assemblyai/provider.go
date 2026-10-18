package assemblyai

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	driver "github.com/peggco/pegg/internal/stt/drivers/assemblyai"
)

const ProviderName = "assemblyai"

func init() {
	stt.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
	baseURL, streamURL := "", ""
	if settings != nil {
		if entry, ok := settings.ProviderConfig(ProviderName); ok {
			baseURL = entry.BaseURL
			streamURL = entry.StreamURL
		}
	}
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}
	return driver.New(driver.ServiceConfig{
		APIKey:    cfg.APIKey,
		BaseURL:   baseURL,
		StreamURL: streamURL,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}), nil
}
