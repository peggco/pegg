package llm

import "github.com/peggco/pegg/internal/core/config"

type Model = config.Model

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonNull          FinishReason = "null"
)

type ListModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type ToolChoice struct {
	Type     string         `json:"type,omitempty"`
	Function ToolChoiceFunc `json:"function,omitempty"`
}

type ToolChoiceFunc struct {
	Name string `json:"name"`
}
