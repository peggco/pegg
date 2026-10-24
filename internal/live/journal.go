package live

import (
	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

type AgentEvent struct {
	Type          string
	AgentID       string
	AgentName     string
	ParentAgentID string

	Input       string
	Attachments []llm.Attachment

	Content   string
	Reasoning string

	Message llm.Message

	ToolCall   llm.ToolCall
	CallID     string
	ToolName   string
	ToolOutput string
	ToolError  string

	Usage llm.Usage

	Questions []agent.AskQuestion
	Answers   map[string]string

	Output       string
	Iterations   int
	FinishReason llm.FinishReason
	Strategy     string
	Messages     int
	Tokens       int

	DisplayName     string
	Model           string
	Provider        string
	ReasoningEffort string

	SubAgentName string
	TaskIDs      []string
	SubAgentOut  string
	SubAgentErr  string
}

func DecodeAgentEvent(e session.RunEvent) (AgentEvent, bool) {
	ev := AgentEvent{
		Type:          e.Type,
		AgentID:       e.AgentID,
		AgentName:     e.AgentName,
		ParentAgentID: e.ParentAgentID,
	}
	switch e.Type {
	case session.AgentEventInput:
		var d struct {
			Input       string           `json:"input"`
			Attachments []llm.Attachment `json:"attachments"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Input = d.Input
		ev.Attachments = d.Attachments
	case session.AgentEventStarted:
		var d struct {
			AgentName       string `json:"agent_name"`
			DisplayName     string `json:"display_name"`
			ParentAgentID   string `json:"parent_agent_id"`
			Provider        string `json:"provider"`
			Model           string `json:"model"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.AgentName = d.AgentName
		ev.DisplayName = d.DisplayName
		if d.ParentAgentID != "" {
			ev.ParentAgentID = d.ParentAgentID
		}
		ev.Provider = d.Provider
		ev.Model = d.Model
		ev.ReasoningEffort = d.ReasoningEffort
	case session.AgentEventToken:
		var d struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Content = d.Content
		ev.Reasoning = d.Reasoning
	case session.AgentEventMessage:
		var d struct {
			Message llm.Message `json:"message"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Message = d.Message
	case session.AgentEventToolCall:
		var d struct {
			CallID   string       `json:"call_id"`
			ToolName string       `json:"tool_name"`
			Args     string       `json:"args"`
			Call     llm.ToolCall `json:"call"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.ToolCall = d.Call
		if ev.ToolCall.ID == "" {
			ev.ToolCall.ID = d.CallID
		}
		if ev.ToolCall.Function.Name == "" {
			ev.ToolCall.Function.Name = d.ToolName
		}
		if ev.ToolCall.Function.Arguments == "" {
			ev.ToolCall.Function.Arguments = d.Args
		}
		ev.CallID = ev.ToolCall.ID
		ev.ToolName = ev.ToolCall.Function.Name
	case session.AgentEventToolResult:
		var d struct {
			CallID   string `json:"call_id"`
			ToolName string `json:"tool_name"`
			Output   string `json:"output"`
			Error    string `json:"error"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.CallID = d.CallID
		ev.ToolName = d.ToolName
		ev.ToolOutput = d.Output
		ev.ToolError = d.Error
		ev.ToolCall = llm.ToolCall{ID: d.CallID}
		ev.ToolCall.Function.Name = d.ToolName
	case session.AgentEventUsage:
		var d struct {
			Usage llm.Usage `json:"usage"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Usage = d.Usage
	case session.AgentEventAsk:
		var d struct {
			AgentName string              `json:"agent_name"`
			Questions []agent.AskQuestion `json:"questions"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.AgentName = d.AgentName
		ev.Questions = d.Questions
	case session.RunEventAskResolved:
		var d struct {
			Answers map[string]string `json:"answers"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Answers = d.Answers
	case session.AgentEventFinished:
		var d struct {
			Output       string           `json:"output"`
			Usage        llm.Usage        `json:"usage"`
			Iterations   int              `json:"iterations"`
			FinishReason llm.FinishReason `json:"finish_reason"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Output = d.Output
		ev.Usage = d.Usage
		ev.Iterations = d.Iterations
		ev.FinishReason = d.FinishReason
	case session.AgentEventError, session.AgentEventErrorMessage:
		var d struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.ToolError = d.Message
	case session.AgentEventErrorMessageFinish:
	case session.AgentEventSubAgentNotice:
		var d struct {
			ParentAgentID string   `json:"parent_agent_id"`
			SubAgentName  string   `json:"subagent_name"`
			TaskIDs       []string `json:"task_ids"`
			Output        string   `json:"output"`
			Error         string   `json:"error"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		if d.ParentAgentID != "" {
			ev.ParentAgentID = d.ParentAgentID
		}
		ev.SubAgentName = d.SubAgentName
		ev.TaskIDs = d.TaskIDs
		ev.SubAgentOut = d.Output
		ev.SubAgentErr = d.Error
	case session.AgentEventCompaction:
		var d struct {
			Strategy string `json:"strategy"`
			Messages int    `json:"messages"`
			Tokens   int    `json:"tokens"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return ev, false
		}
		ev.Strategy = d.Strategy
		ev.Messages = d.Messages
		ev.Tokens = d.Tokens
	default:
		return ev, false
	}
	return ev, true
}

func DecodeRunState(e session.RunEvent) (session.RunState, string, bool) {
	if e.Type != session.RunEventState {
		return "", "", false
	}
	var d struct {
		State string `json:"state"`
		Error string `json:"error"`
	}
	if json.Unmarshal(e.Data, &d) != nil {
		return "", "", false
	}
	return session.RunState(d.State), d.Error, true
}
