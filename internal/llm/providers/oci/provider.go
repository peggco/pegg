package oci

import (
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	openaidriver "github.com/peggco/pegg/internal/llm/drivers/openai"
)

const (
	ProviderName   = "oci"
	DefaultBaseURL = "https://inference.generativeai.us-chicago-1.oci.oraclecloud.com/openai/v1"
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
