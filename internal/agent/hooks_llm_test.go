package agent

import (
	"context"
	"strings"
	"testing"

	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/llm"
)

func withAgentHooks(t *testing.T) {
	t.Helper()
	corehook.SetDefault(corehook.NewBus())
	t.Cleanup(corehook.ResetDefault)
}

func TestLLMRequestHookRewritesModel(t *testing.T) {
	withAgentHooks(t)
	corehook.Default().Registry().AddFilterFor("test", llm.HookRequest, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		if request, ok := payload["request"].(map[string]any); ok {
			request["model"] = "hooked-model"
		}
		return payload, nil
	})
	a, prov := newTestAgent(t)
	prov.responses = []mockResponse{{content: "ok"}}
	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if prov.lastReq == nil || prov.lastReq.Model != "hooked-model" {
		t.Fatalf("request model: %#v", prov.lastReq)
	}
}

func TestLLMResponseHookRewritesOutput(t *testing.T) {
	withAgentHooks(t)
	corehook.Default().Registry().AddFilterFor("test", llm.HookResponse, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		response, ok := payload["response"].(map[string]any)
		if !ok {
			return payload, nil
		}
		choices, ok := response["choices"].([]any)
		if !ok || len(choices) == 0 {
			return payload, nil
		}
		choice := choices[0].(map[string]any)
		message := choice["message"].(map[string]any)
		message["content"] = "hooked"
		return payload, nil
	})
	a, prov := newTestAgent(t)
	prov.responses = []mockResponse{{content: "original"}}
	res, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "hooked" {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestLLMStreamChunkHookRewritesContent(t *testing.T) {
	withAgentHooks(t)
	corehook.Default().Registry().AddFilterFor("test", llm.HookStreamChunk, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		if payload["content"] == "Hel" {
			payload["content"] = "H"
		}
		return payload, nil
	})
	a, prov := newTestAgent(t)
	prov.streams = [][]llm.StreamChunk{{
		{Content: "Hel"},
		{Content: "lo"},
		{FinishReason: llm.FinishReasonStop, IsDone: true},
	}}
	res, err := a.RunStream(context.Background(), "hi", func(StreamEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "Hlo" {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestToolBeforeHookBlocksCall(t *testing.T) {
	withAgentHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookToolBefore, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["blocked"] = true
		payload["reason"] = "policy blocked"
		return payload, nil
	})
	a, prov := newTestAgent(t, WithTool(echoTool()))
	prov.responses = []mockResponse{
		{calls: []llm.ToolCall{toolCall("c1", "echo", `{"msg":"x"}`)}},
		{content: "done"},
	}
	res, err := a.Run(context.Background(), "echo x")
	if err != nil {
		t.Fatal(err)
	}
	got := toolMessageContent(res.History, "c1")
	if !strings.Contains(got, "policy blocked") {
		t.Fatalf("tool content = %q", got)
	}
	if strings.Contains(got, "echo:") {
		t.Fatalf("tool executed despite block: %q", got)
	}
}

func TestToolAfterHookRewritesOutput(t *testing.T) {
	withAgentHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookToolAfter, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["output"] = "rewritten"
		return payload, nil
	})
	a, prov := newTestAgent(t, WithTool(echoTool()))
	prov.responses = []mockResponse{
		{calls: []llm.ToolCall{toolCall("c1", "echo", `{"msg":"x"}`)}},
		{content: "done"},
	}
	res, err := a.Run(context.Background(), "echo x")
	if err != nil {
		t.Fatal(err)
	}
	if got := toolMessageContent(res.History, "c1"); got != "rewritten" {
		t.Fatalf("tool content = %q", got)
	}
}

func toolMessageContent(history []llm.Message, callID string) string {
	for _, msg := range history {
		if msg.Role == llm.RoleTool && msg.ToolCallID == callID {
			return llm.MessageText(msg)
		}
	}
	return ""
}
