package agent

import (
	"github.com/peggco/pegg/internal/agent/middleware"
	"github.com/peggco/pegg/internal/llm"
)

type RunResult = middleware.Result

type StreamEventType string

const (
	StreamToken      StreamEventType = "token"
	StreamReasoning  StreamEventType = "reasoning"
	StreamToolCall   StreamEventType = "tool_call"
	StreamToolOutput StreamEventType = "tool_output"
	StreamToolResult StreamEventType = "tool_result"
	StreamCompaction StreamEventType = "compaction"
	StreamDone       StreamEventType = "done"
)

type StreamEvent struct {
	Type            StreamEventType
	AgentID         string
	AgentName       string
	Model           llm.Model
	Content         string
	Reasoning       string
	ToolCall        *llm.ToolCall
	ToolCallID      string
	ToolName        string
	ToolStream      string
	ToolChunk       string
	ToolOutput      string
	ToolErr         error
	ToolAttachments []llm.Attachment
	Usage           *llm.Usage
	Strategy        string
	Messages        int
	Tokens          int
}

type StreamHandler func(StreamEvent) error
