package assemblyai

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

const DriverName = "assemblyai"

func init() {
	stt.RegisterDriver(DriverName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
	name := cfg.Provider
	if name == "" {
		name = DriverName
	}
	baseURL, streamURL := cfg.BaseURL, ""
	if settings != nil {
		if entry, ok := settings.ProviderConfig(name); ok {
			if baseURL == "" {
				baseURL = entry.BaseURL
			}
			streamURL = entry.StreamURL
		}
	}
	return New(ServiceConfig{
		Name:      name,
		APIKey:    cfg.APIKey,
		BaseURL:   baseURL,
		StreamURL: streamURL,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Models:    stt.ModelsForConfig(name, settings),
	}), nil
}
