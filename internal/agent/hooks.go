package agent

import (
	"context"

	"github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/llm"
)

type MessageInput struct {
	Text            string
	Calls           []llm.ToolCall
	SystemReminders []string
}

var messageInputHook = hook.NewHook[MessageInput]()

func OnMessageInput(fn func(MessageInput) MessageInput) {
	messageInputHook.Add(fn)
}

func expandInput(input string) MessageInput {
	return messageInputHook.Apply(MessageInput{Text: input})
}

type ModelResolve struct {
	Ctx   context.Context
	Agent *Agent
	Input string
}

var ModelResolveHook = hook.NewHook[ModelResolve]()

func OnModelResolve(fn func(ModelResolve) ModelResolve) {
	ModelResolveHook.Add(fn)
}

func resolveModel(ctx context.Context, a *Agent, input string) {
	ModelResolveHook.Apply(ModelResolve{Ctx: ctx, Agent: a, Input: input})
}

const (
	HookToolBefore     = "tool.before"
	HookToolAfter      = "tool.after"
	HookToolPermission = "tool.permission"
)

type ToolBeforeHook struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
	Blocked   bool   `json:"blocked,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ToolAfterHook struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ToolPermissionHook struct {
	AgentID   string `json:"agent_id"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments,omitempty"`
	Decision  string `json:"decision,omitempty"`
	Reason    string `json:"reason,omitempty"`
}
