package llm

import "encoding/json"

const (
	HookRequest       = "llm.request"
	HookResponse      = "llm.response"
	HookStreamChunk   = "llm.stream.chunk"
	HookModelResolved = "llm.model.resolved"
	HookUsage         = "llm.usage"
	HookError         = "llm.error"
)

type RequestHook struct {
	AgentID   string  `json:"agent_id"`
	AgentName string  `json:"agent_name"`
	Request   Request `json:"request"`
}

type ResponseHook struct {
	AgentID   string   `json:"agent_id"`
	AgentName string   `json:"agent_name"`
	Request   Request  `json:"request"`
	Response  Response `json:"response"`
}

type StreamChunkHook struct {
	AgentID      string       `json:"agent_id"`
	Model        string       `json:"model,omitempty"`
	Content      string       `json:"content,omitempty"`
	Reasoning    string       `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall   `json:"tool_calls,omitempty"`
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	Done         bool         `json:"done,omitempty"`
}

type ModelResolvedHook struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Input     string `json:"input,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
}

type UsageHook struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Model     string `json:"model,omitempty"`
	Usage     Usage  `json:"usage"`
}

type ErrorHook struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error"`
}

func (r *Request) Normalize() {
	if r == nil {
		return
	}
	for i := range r.Messages {
		r.Messages[i].Normalize()
	}
}

func (r *Response) Normalize() {
	if r == nil {
		return
	}
	for i := range r.Choices {
		if r.Choices[i].Message != nil {
			r.Choices[i].Message.Normalize()
		}
	}
}

func (m *Message) Normalize() {
	m.Content = coerceContent(m.Content)
	m.Reasoning = coerceContent(m.Reasoning)
}

func coerceContent(value any) any {
	obj, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if _, hasText := obj["text"]; !hasText {
		if _, hasAttachments := obj["attachments"]; !hasAttachments {
			return value
		}
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return value
	}
	var content Content
	if err := json.Unmarshal(raw, &content); err != nil {
		return value
	}
	return content
}
