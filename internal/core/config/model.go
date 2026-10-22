package config

import (
	json "github.com/goccy/go-json"
)

type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`

	Config *ModelConfig `json:"config,omitempty"`
}

func (m *Model) MaxInputTokens() int {
	if m == nil || m.Config == nil {
		return 0
	}
	if m.Config.MaxInputTokens > 0 {
		return m.Config.MaxInputTokens
	}
	return m.Config.MaxTokens
}

type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    *int     `json:"min,omitempty"`
	Max    *int     `json:"max,omitempty"`
}

type Interleaved struct {
	Field string `json:"field"`
}

func (i *Interleaved) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			i.Field = "default"
		}
		return nil
	}
	type alias Interleaved
	return json.Unmarshal(data, (*alias)(i))
}

type Modalities struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

func (c *ModelConfig) Normalize() {
	if c == nil {
		return
	}
	if len(c.ReasoningOptions) > 0 && !c.SupportsReasoning {
		c.SupportsReasoning = true
	}
}

func (c *ModelConfig) ReasoningEfforts() []string {
	if c == nil {
		return nil
	}
	for _, opt := range c.ReasoningOptions {
		if opt.Type == "effort" {
			return opt.Values
		}
	}
	return nil
}

type ModelConfig struct {
	MaxTokens       int `json:"max_tokens,omitempty"`
	MaxInputTokens  int `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	InputCostPerToken     float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken    float64 `json:"output_cost_per_token,omitempty"`
	CacheReadCostPerToken float64 `json:"cache_read_cost_per_token,omitempty"`

	SupportsReasoning bool `json:"supports_reasoning,omitempty"`

	Family      string `json:"family,omitempty"`
	Description string `json:"description,omitempty"`
	Knowledge   string `json:"knowledge,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
	OpenWeights bool   `json:"open_weights,omitempty"`
	Attachment  bool   `json:"attachment,omitempty"`
	ToolCall    bool   `json:"tool_call,omitempty"`

	StructuredOutput bool              `json:"structured_output,omitempty"`
	Temperature      bool              `json:"temperature,omitempty"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options,omitempty"`
	Interleaved      *Interleaved      `json:"interleaved,omitempty"`
	Modalities       *Modalities       `json:"modalities,omitempty"`
}
