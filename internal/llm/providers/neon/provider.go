package neon

import (
	"fmt"
	"os"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	openaidriver "github.com/peggco/pegg/internal/llm/drivers/openai"
)

const ProviderName = "neon"

func init() {
	llm.RegisterProvider(ProviderName, NewFromConfig)
}

func NewFromConfig(cfg config.LLMConfig) (llm.Provider, error) {
	baseURL, err := defaultBaseURL()
	if err != nil {
		return nil, err
	}
	return openaidriver.NewService(ProviderName, openaidriver.ServiceConfig{
		BaseURL: baseURL,
		APIKey:  cfg.APIKey,
		Headers: cfg.Headers,
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}), nil
}

func defaultBaseURL() (string, error) {
	var missing string
	baseURL := os.Expand("${NEON_AI_GATEWAY_BASE_URL}/v1", func(key string) string {
		v := os.Getenv(key)
		if v == "" && missing == "" {
			missing = key
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("%s: missing environment variable %s", ProviderName, missing)
	}
	return baseURL, nil
}
