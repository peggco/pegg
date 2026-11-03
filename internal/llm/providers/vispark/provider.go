package vispark

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	openaidriver "github.com/peggco/pegg/internal/llm/drivers/openai"
)

const (
	ProviderName   = "vispark"
	DefaultBaseURL = "https://api.lab.vispark.in/v1"
)

func init() {
	llm.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig) (llm.Provider, error) {
	return openaidriver.NewService(ProviderName, openaidriver.ServiceConfig{
		BaseURL: DefaultBaseURL,
		APIKey:  cfg.APIKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}), nil
}
