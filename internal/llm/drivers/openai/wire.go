package openai

import (
	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/llm"
)

type chatMessage struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	Reasoning        json.RawMessage `json:"reasoning,omitempty"`
	ReasoningContent json.RawMessage `json:"reasoning_content,omitempty"`
	Name             string          `json:"name,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	ToolCalls        []chatToolCall  `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model             string            `json:"model"`
	Messages          []chatMessage     `json:"messages"`
	Temperature       float64           `json:"temperature,omitempty"`
	TopP              float64           `json:"top_p,omitempty"`
	MaxTokens         int               `json:"max_tokens,omitempty"`
	Stream            bool              `json:"stream,omitempty"`
	StreamOptions     *streamOptions    `json:"stream_options,omitempty"`
	Tools             []json.RawMessage `json:"tools,omitempty"`
	ToolChoice        any               `json:"tool_choice,omitempty"`
	ResponseFormat    json.RawMessage   `json:"response_format,omitempty"`
	ParallelToolCalls bool              `json:"parallel_tool_calls,omitempty"`
	N                 int               `json:"n,omitempty"`
	PresencePenalty   float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty  float64           `json:"frequency_penalty,omitempty"`
	User              string            `json:"user,omitempty"`
	ReasoningEffort   string            `json:"reasoning_effort,omitempty"`
}

type chatResponse struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"`
	Created           int64        `json:"created"`
	Model             string       `json:"model"`
	Choices           []chatChoice `json:"choices"`
	Usage             chatUsage    `json:"usage"`
	SystemFingerprint string       `json:"system_fingerprint,omitempty"`
}

type chatChoice struct {
	Index        int          `json:"index"`
	Message      *chatMessage `json:"message,omitempty"`
	Delta        *chatDelta   `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason,omitempty"`
}

type chatDelta struct {
	Role             *string       `json:"role,omitempty"`
	Content          *string       `json:"content,omitempty"`
	Reasoning        *string       `json:"reasoning,omitempty"`
	ReasoningContent *string       `json:"reasoning_content,omitempty"`
	ToolCalls        []chatDeltaTC `json:"tool_calls,omitempty"`
	Refusal          *string       `json:"refusal,omitempty"`
}

type chatDeltaTC struct {
	Index    int      `json:"index"`
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Function chatFunc `json:"function,omitempty"`
}

type chatFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatUsage struct {
	PromptTokens            int                    `json:"prompt_tokens"`
	CompletionTokens        int                    `json:"completion_tokens"`
	TotalTokens             int                    `json:"total_tokens"`
	Cost                    float64                `json:"cost,omitempty"`
	IsByok                  bool                   `json:"is_byok,omitempty"`
	PromptTokensDetails     *chatPromptDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *chatCompletionDetails `json:"completion_tokens_details,omitempty"`
	CostDetails             *chatCostDetails       `json:"cost_details,omitempty"`
}

type chatPromptDetails struct {
	CachedTokens     int `json:"cached_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	AudioTokens      int `json:"audio_tokens,omitempty"`
	VideoTokens      int `json:"video_tokens,omitempty"`
}

type chatCompletionDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	ImageTokens     int `json:"image_tokens,omitempty"`
	AudioTokens     int `json:"audio_tokens,omitempty"`
}

type chatCostDetails struct {
	UpstreamInferenceCost            float64 `json:"upstream_inference_cost,omitempty"`
	UpstreamInferencePromptCost      float64 `json:"upstream_inference_prompt_cost,omitempty"`
	UpstreamInferenceCompletionsCost float64 `json:"upstream_inference_completions_cost,omitempty"`
}

type chatModelsResponse struct {
	Object string      `json:"object"`
	Data   []chatModel `json:"data"`
}

type chatModel struct {
	ID      string           `json:"id"`
	Name    string           `json:"name,omitempty"`
	Object  string           `json:"object,omitempty"`
	Created int64            `json:"created,omitempty"`
	OwnedBy string           `json:"owned_by,omitempty"`
	Config  *llm.ModelConfig `json:"config,omitempty"`
}
