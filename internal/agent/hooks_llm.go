package agent

import (
	"context"
	"time"

	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/llm"
)

const streamChunkHookTimeout = 100 * time.Millisecond

func (a *Agent) publishModelResolved(ctx context.Context, input string) {
	provider := ""
	if a.Provider != nil {
		provider = a.Provider.Name()
	}
	corehook.ApplyAction(ctx, llm.HookModelResolved, llm.ModelResolvedHook{
		AgentID:   a.ID,
		AgentName: a.Name,
		Input:     input,
		Provider:  provider,
		Model:     a.Model.ID,
	})
}

func (a *Agent) applyRequestHook(ctx context.Context, req *llm.Request) {
	if req == nil {
		return
	}
	hooked, errs := corehook.ApplyFilter(ctx, llm.HookRequest, llm.RequestHook{
		AgentID:   a.ID,
		AgentName: a.Name,
		Request:   *req,
	})
	if len(errs) > 0 {
		a.debugf("agent %q llm.request hook: %v", a.Name, errs)
	}
	*req = hooked.Request
	req.Normalize()
}

func (a *Agent) applyResponseHook(ctx context.Context, req *llm.Request, resp *llm.Response) *llm.Response {
	if resp == nil || req == nil {
		return resp
	}
	hooked, errs := corehook.ApplyFilter(ctx, llm.HookResponse, llm.ResponseHook{
		AgentID:   a.ID,
		AgentName: a.Name,
		Request:   *req,
		Response:  *resp,
	})
	if len(errs) > 0 {
		a.debugf("agent %q llm.response hook: %v", a.Name, errs)
	}
	out := hooked.Response
	out.Normalize()
	return &out
}

func (a *Agent) applyChunkHook(ctx context.Context, model string, chunk llm.StreamChunk) llm.StreamChunk {
	chunkCtx, cancel := context.WithTimeout(ctx, streamChunkHookTimeout)
	defer cancel()
	hooked, errs := corehook.ApplyFilter(chunkCtx, llm.HookStreamChunk, llm.StreamChunkHook{
		AgentID:      a.ID,
		Model:        model,
		Content:      chunk.Content,
		Reasoning:    chunk.Reasoning,
		ToolCalls:    chunk.ToolCalls,
		FinishReason: chunk.FinishReason,
		Done:         chunk.IsDone,
	})
	if len(errs) > 0 {
		return chunk
	}
	chunk.Content = hooked.Content
	chunk.Reasoning = hooked.Reasoning
	chunk.ToolCalls = hooked.ToolCalls
	if hooked.FinishReason != "" {
		chunk.FinishReason = hooked.FinishReason
	}
	return chunk
}

func (a *Agent) publishUsage(ctx context.Context, state *runState) {
	if state == nil {
		return
	}
	a.publish(TopicAgentUsage, AgentUsage{
		AgentID:   a.ID,
		AgentName: a.Name,
		Model:     a.Model,
		Usage:     state.usage,
	})
	corehook.ApplyAction(ctx, llm.HookUsage, llm.UsageHook{
		AgentID:   a.ID,
		AgentName: a.Name,
		Model:     a.Model.ID,
		Usage:     state.usage,
	})
}

func (a *Agent) publishHookError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	corehook.ApplyAction(ctx, llm.HookError, llm.ErrorHook{
		AgentID:   a.ID,
		AgentName: a.Name,
		Model:     a.Model.ID,
		Error:     err.Error(),
	})
}
