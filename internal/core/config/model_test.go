package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProviderModelsRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	models := []Model{
		{
			ID:   "gpt-4o",
			Name: "GPT-4o",
			Config: &ModelConfig{
				MaxInputTokens:        128000,
				MaxOutputTokens:       16384,
				InputCostPerToken:     0.0000025,
				OutputCostPerToken:    0.00001,
				ToolCall:              true,
				StructuredOutput:      true,
				Temperature:           true,
				SupportsReasoning:     false,
				Modalities:            &Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
				ReasoningOptions:      []ReasoningOption{{Type: "effort", Values: []string{"low", "medium", "high"}}},
				Family:                "gpt",
				Description:           "Test model",
				Knowledge:             "2023-10",
				ReleaseDate:           "2024-05-13",
				LastUpdated:           "2024-05-13",
				OpenWeights:           false,
				Attachment:            true,
				CacheReadCostPerToken: 0.00000125,
			},
		},
		{ID: "plain-model", Name: "Plain"},
	}

	if err := UpsertProvider(LLMConfig{Provider: "declared", Driver: "openai", BaseURL: "https://x/v1", Models: models}); err != nil {
		t.Fatal(err)
	}

	stored, ok := GetProvider("declared")
	if !ok {
		t.Fatal("provider not found")
	}
	if len(stored.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(stored.Models))
	}
	got := stored.Models[0]
	if got.ID != "gpt-4o" || got.Name != "GPT-4o" {
		t.Fatalf("model = %+v", got)
	}
	if got.Config == nil {
		t.Fatal("config missing")
	}
	if got.Config.MaxInputTokens != 128000 || got.Config.MaxOutputTokens != 16384 {
		t.Fatalf("limits = %+v", got.Config)
	}
	if got.Config.InputCostPerToken != 0.0000025 {
		t.Fatalf("input cost = %v", got.Config.InputCostPerToken)
	}
	if got.Config.Modalities == nil || len(got.Config.Modalities.Input) != 2 {
		t.Fatalf("modalities = %+v", got.Config.Modalities)
	}
	if len(got.Config.ReasoningOptions) != 1 || got.Config.ReasoningOptions[0].Type != "effort" {
		t.Fatalf("reasoning options = %+v", got.Config.ReasoningOptions)
	}
	if stored.Models[1].Config != nil {
		t.Fatalf("plain model must not have a config: %+v", stored.Models[1].Config)
	}

	data, err := os.ReadFile(filepath.Join(home, ".pegg", "pegg.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(data), `"models"`, `"max_input_tokens"`, `"modalities"`, `"input_cost_per_token"`) {
		t.Fatalf("unexpected config file:\n%s", data)
	}
}

func TestModelMaxInputTokens(t *testing.T) {
	var nilModel *Model
	if nilModel.MaxInputTokens() != 0 {
		t.Fatal("nil model must report 0")
	}
	if (&Model{}).MaxInputTokens() != 0 {
		t.Fatal("model without config must report 0")
	}
	if (&Model{Config: &ModelConfig{MaxTokens: 4096}}).MaxInputTokens() != 4096 {
		t.Fatal("max_tokens must be the fallback")
	}
	if (&Model{Config: &ModelConfig{MaxTokens: 4096, MaxInputTokens: 2048}}).MaxInputTokens() != 2048 {
		t.Fatal("max_input_tokens must win")
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		found := false
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestModelConfigNormalizeDerivesSupportsReasoning(t *testing.T) {
	cfg := &ModelConfig{
		ReasoningOptions: []ReasoningOption{{Type: "effort", Values: []string{"low", "high"}}},
	}
	cfg.Normalize()
	if !cfg.SupportsReasoning {
		t.Fatal("reasoning options must imply supports_reasoning")
	}
	efforts := cfg.ReasoningEfforts()
	if len(efforts) != 2 || efforts[0] != "low" || efforts[1] != "high" {
		t.Fatalf("efforts = %v", efforts)
	}
}

func TestModelConfigNormalizeKeepsExplicitFlag(t *testing.T) {
	cfg := &ModelConfig{SupportsReasoning: true}
	cfg.Normalize()
	if !cfg.SupportsReasoning {
		t.Fatal("the explicit flag must be kept")
	}
	if efforts := cfg.ReasoningEfforts(); efforts != nil {
		t.Fatalf("efforts = %v, want none", efforts)
	}
}

func TestModelConfigNormalizeHandlesNil(t *testing.T) {
	var cfg *ModelConfig
	cfg.Normalize()
	if cfg != nil {
		t.Fatal("nil config must stay nil")
	}
}
