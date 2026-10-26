package openai

import (
	"fmt"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

const DriverName = "openai"

func init() {
	stt.RegisterDriver(DriverName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("openai: base_url is required for %q driver", DriverName)
	}
	name := cfg.Provider
	if name == "" {
		name = DriverName
	}
	return New(ServiceConfig{
		Name:    name,
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Models:  stt.ModelsForConfig(name, settings),
	}), nil
}
