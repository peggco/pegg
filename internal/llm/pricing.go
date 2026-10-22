package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

const ModelsDataURL = "https://cdn.pegg.dev/models.json"

const PricesCacheKey = "models_cache"

var ErrModelConfigNotFound = errors.New("llm: model config not found")

type (
	ModelConfig     = config.ModelConfig
	ReasoningOption = config.ReasoningOption
	Interleaved     = config.Interleaved
	Modalities      = config.Modalities
)

type apiProvider struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Models map[string]apiModelEntry `json:"models"`
}

type apiModelEntry struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Family           string            `json:"family"`
	Attachment       bool              `json:"attachment"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options"`
	ToolCall         bool              `json:"tool_call"`
	Interleaved      *Interleaved      `json:"interleaved"`
	StructuredOutput bool              `json:"structured_output"`
	Temperature      bool              `json:"temperature"`
	Knowledge        string            `json:"knowledge"`
	ReleaseDate      string            `json:"release_date"`
	LastUpdated      string            `json:"last_updated"`
	Modalities       *Modalities       `json:"modalities"`
	OpenWeights      bool              `json:"open_weights"`
	Limit            struct {
		Context int `json:"context"`
		Input   int `json:"input"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost struct {
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
		CacheRead float64 `json:"cache_read"`
	} `json:"cost"`
}

func FetchPrices(ctx context.Context) (map[string]ModelConfig, error) {
	return fetchPricesFrom(ctx, ModelsDataURL)
}

func fetchPricesFrom(ctx context.Context, url string) (map[string]ModelConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("llm: create prices request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: fetch prices: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm: fetch prices: status %d", resp.StatusCode)
	}

	var providers map[string]apiProvider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, fmt.Errorf("llm: decode prices: %w", err)
	}

	out := make(map[string]ModelConfig)
	for _, prov := range providers {
		for modelID, entry := range prov.Models {
			cfg := ModelConfig{
				MaxInputTokens:  entry.Limit.Input,
				MaxOutputTokens: entry.Limit.Output,
				MaxTokens:       entry.Limit.Context,

				InputCostPerToken:     entry.Cost.Input / 1_000_000,
				OutputCostPerToken:    entry.Cost.Output / 1_000_000,
				CacheReadCostPerToken: entry.Cost.CacheRead / 1_000_000,

				SupportsReasoning: entry.Reasoning,

				Family:      entry.Family,
				Description: entry.Description,
				Knowledge:   entry.Knowledge,
				ReleaseDate: entry.ReleaseDate,
				LastUpdated: entry.LastUpdated,
				OpenWeights: entry.OpenWeights,
				Attachment:  entry.Attachment,
				ToolCall:    entry.ToolCall,

				StructuredOutput: entry.StructuredOutput,
				Temperature:      entry.Temperature,
				ReasoningOptions: entry.ReasoningOptions,
				Interleaved:      entry.Interleaved,
				Modalities:       entry.Modalities,
			}
			out[modelID] = cfg
			out[prov.ID+"/"+modelID] = cfg
		}
	}
	return out, nil
}

func lookupModelConfig(prices map[string]ModelConfig, provider, model string) (*ModelConfig, error) {
	if cfg, ok := prices[provider+"/"+model]; ok {
		return &cfg, nil
	}
	if cfg, ok := prices[model]; ok {
		return &cfg, nil
	}
	for key, cfg := range prices {
		if strings.Contains(key, model) {
			return &cfg, nil
		}
	}
	return nil, ErrModelConfigNotFound
}
