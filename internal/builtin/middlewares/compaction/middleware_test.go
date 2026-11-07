package compaction

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
)

func TestInvokeToolTruncates(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:            true,
			Strategy:           []string{"tool-clearing"},
			MaxToolOutputChars: 20,
		},
	})

	longOutput := strings.Repeat("x", 100)
	next := func(ctx context.Context, call llm.ToolCall) (string, error) {
		return longOutput, nil
	}

	output, err := m.InvokeTool(context.Background(), llm.ToolCall{}, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output) > 40 {
		t.Errorf("output not truncated: got %d chars, want <= 40", len(output))
	}
	if !strings.HasSuffix(output, "... [truncated]") {
		t.Errorf("output missing truncation suffix: %s", output)
	}
}

func TestInvokeToolNoTruncationWhenDisabled(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:            false,
			Strategy:           []string{"tool-clearing"},
			MaxToolOutputChars: 20,
		},
	})

	longOutput := strings.Repeat("x", 100)
	next := func(ctx context.Context, call llm.ToolCall) (string, error) {
		return longOutput, nil
	}

	output, _ := m.InvokeTool(context.Background(), llm.ToolCall{}, next)
	if output != longOutput {
		t.Errorf("output should not be truncated when disabled")
	}
}

func TestInvokeToolNoTruncationWhenShort(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:            true,
			Strategy:           []string{"tool-clearing"},
			MaxToolOutputChars: 100,
		},
	})

	shortOutput := "hello"
	next := func(ctx context.Context, call llm.ToolCall) (string, error) {
		return shortOutput, nil
	}

	output, _ := m.InvokeTool(context.Background(), llm.ToolCall{}, next)
	if output != shortOutput {
		t.Errorf("short output should not be truncated")
	}
}

func TestBeforeLLMToolClearing(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:            true,
			Strategy:           []string{"tool-clearing"},
			MaxToolOutputChars: 10,
		},
	})

	longToolOutput := strings.Repeat("y", 50)
	req := &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "system"},
			{Role: llm.RoleUser, Content: "user msg"},
			{Role: llm.RoleAssistant, Content: "assistant msg", ToolCalls: []llm.ToolCall{{ID: "1", Function: llm.Function{Name: "read"}}}},
			{Role: llm.RoleTool, Content: longToolOutput, ToolCallID: "1"},
		},
	}

	err := m.BeforeLLM(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolMsg := req.Messages[3]
	text := llm.MessageText(toolMsg)
	if len(text) > 30 {
		t.Errorf("tool message not truncated: got %d chars", len(text))
	}
}

func TestBeforeLLMSlidingWindow(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:     true,
			Strategy:    []string{"sliding-window"},
			MaxMessages: 3,
		},
	})

	msgs := make([]llm.Message, 0, 10)
	msgs = append(msgs, llm.SystemMessage("system prompt"))
	for i := 0; i < 8; i++ {
		msgs = append(msgs, llm.UserMessage("user msg"))
		msgs = append(msgs, llm.AssistantMessage("assistant msg"))
	}

	ref := &msgs
	ctx := agent.WithHistory(context.Background(), ref)

	req := &llm.Request{Messages: msgs}

	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()

	err := m.BeforeLLM(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(req.Messages) > 5 {
		t.Errorf("sliding window did not compact: got %d messages, want <= 5", len(req.Messages))
	}

	if req.Messages[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %v", req.Messages[0].Role)
	}

	if !strings.Contains(llm.MessageText(req.Messages[1]), "compacted") {
		t.Errorf("second message should be compaction notice")
	}
}

func TestAfterLLMDetectsPressure(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:   true,
			Strategy:  []string{"sliding-window"},
			Threshold: 80,
		},
	})

	a := agent.New("test",
		agent.WithModel(llm.Model{
			ID: "test-model",
			Config: &llm.ModelConfig{
				MaxInputTokens: 1000,
			},
		}),
	)

	ctx := agent.WithAgent(context.Background(), a)
	req := &llm.Request{}
	resp := &llm.Response{
		Usage: llm.Usage{
			PromptTokens: 900,
		},
	}

	err := m.AfterLLM(ctx, req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m.mu.Lock()
	needs := m.needsCompact
	m.mu.Unlock()

	if !needs {
		t.Error("AfterLLM should detect pressure when prompt tokens exceed threshold")
	}
}

func TestAfterLLMNoPressureBelowThreshold(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:   true,
			Strategy:  []string{"sliding-window"},
			Threshold: 80,
		},
	})

	a := agent.New("test",
		agent.WithModel(llm.Model{
			ID: "test-model",
			Config: &llm.ModelConfig{
				MaxInputTokens: 1000,
			},
		}),
	)

	ctx := agent.WithAgent(context.Background(), a)
	req := &llm.Request{}
	resp := &llm.Response{
		Usage: llm.Usage{
			PromptTokens: 500,
		},
	}

	_ = m.AfterLLM(ctx, req, resp)

	m.mu.Lock()
	needs := m.needsCompact
	m.mu.Unlock()

	if needs {
		t.Error("AfterLLM should not detect pressure below threshold")
	}
}

func TestHasStrategy(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Strategy: []string{"tool-clearing", "sliding-window"},
		},
	})

	if !m.hasStrategy("tool-clearing") {
		t.Error("should have tool-clearing strategy")
	}
	if !m.hasStrategy("sliding-window") {
		t.Error("should have sliding-window strategy")
	}
	if m.hasStrategy("summarization") {
		t.Error("should not have summarization strategy")
	}
}

func TestFormatHistoryForSummary(t *testing.T) {
	history := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are helpful"},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "hi there"},
		{Role: llm.RoleUser, Content: "read file.go"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Function: llm.Function{Name: "read"}}}},
		{Role: llm.RoleTool, Content: "file contents here", ToolCallID: "1"},
	}

	result := formatHistoryForSummary(history)

	if strings.Contains(result, "you are helpful") {
		t.Error("summary input should skip system messages")
	}
	if !strings.Contains(result, "user: hello") {
		t.Error("summary input should include user messages")
	}
	if !strings.Contains(result, "assistant: hi there") {
		t.Error("summary input should include assistant text")
	}
	if !strings.Contains(result, "tool call: read") {
		t.Error("summary input should include tool calls")
	}
}

func TestBeforeLLMNoopWhenDisabled(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled: false,
		},
	})

	original := []llm.Message{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: "user"},
	}

	ref := &original
	ctx := agent.WithHistory(context.Background(), ref)
	req := &llm.Request{Messages: original}

	_ = m.BeforeLLM(ctx, req)

	if len(req.Messages) != 2 {
		t.Errorf("BeforeLLM should not modify messages when disabled")
	}
}

func TestBeforeLLMNoopWhenNoHistoryRef(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:  true,
			Strategy: []string{"sliding-window"},
		},
	})

	req := &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
	}

	err := m.BeforeLLM(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaxToolOutputCharsDefault(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:            true,
			Strategy:           []string{"tool-clearing"},
			MaxToolOutputChars: 0,
		},
	})

	longOutput := strings.Repeat("z", 5000)
	next := func(ctx context.Context, call llm.ToolCall) (string, error) {
		return longOutput, nil
	}

	output, _ := m.InvokeTool(context.Background(), llm.ToolCall{}, next)
	if len(output) > 4020 {
		t.Errorf("should use default 4000 char limit, got %d", len(output))
	}
}

func TestSlidingWindowPreservesSystemAndRecent(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:     true,
			Strategy:    []string{"sliding-window"},
			MaxMessages: 2,
		},
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "msg 1"},
		{Role: llm.RoleAssistant, Content: "reply 1"},
		{Role: llm.RoleUser, Content: "msg 2"},
		{Role: llm.RoleAssistant, Content: "reply 2"},
		{Role: llm.RoleUser, Content: "msg 3"},
		{Role: llm.RoleAssistant, Content: "reply 3"},
		{Role: llm.RoleUser, Content: "msg 4"},
		{Role: llm.RoleAssistant, Content: "reply 4"},
	}

	ref := &msgs
	ctx := agent.WithHistory(context.Background(), ref)
	req := &llm.Request{Messages: msgs}

	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()

	_ = m.BeforeLLM(ctx, req)

	if len(req.Messages) != 4 {
		t.Errorf("expected 4 messages (system + compact notice + 2 recent), got %d", len(req.Messages))
		return
	}

	lastTwo := req.Messages[2:]
	for _, msg := range lastTwo {
		text := llm.MessageText(msg)
		if text == "msg 3" || text == "reply 3" || text == "msg 4" || text == "reply 4" {
			continue
		}
		t.Errorf("recent messages should be preserved, got: %s", text)
	}
}

func TestSlidingWindowPreservesFirstSystemMessage(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:     true,
			Strategy:    []string{"sliding-window"},
			MaxMessages: 2,
		},
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleSystem, Content: "system reminder"},
		{Role: llm.RoleUser, Content: "msg 1"},
		{Role: llm.RoleAssistant, Content: "reply 1"},
		{Role: llm.RoleUser, Content: "msg 2"},
		{Role: llm.RoleAssistant, Content: "reply 2"},
		{Role: llm.RoleUser, Content: "msg 3"},
		{Role: llm.RoleAssistant, Content: "reply 3"},
	}
	ref := &msgs
	ctx := agent.WithHistory(context.Background(), ref)
	req := &llm.Request{Messages: msgs}

	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()

	_ = m.BeforeLLM(ctx, req)

	if len(req.Messages) != 4 {
		t.Fatalf("messages = %d, want 4 (system + notice + 2 recent): %+v", len(req.Messages), req.Messages)
	}
	if llm.MessageText(req.Messages[0]) != "system prompt" {
		t.Fatalf("first system message was not preserved: %+v", req.Messages[0])
	}
	if !strings.Contains(llm.MessageText(req.Messages[1]), "compacted") {
		t.Fatalf("compaction notice = %q", llm.MessageText(req.Messages[1]))
	}
	for _, msg := range req.Messages {
		if llm.MessageText(msg) == "system reminder" {
			t.Fatal("later system reminders should be discardable")
		}
	}
}

func TestSlidingWindowUsesContextWindowSize(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:   true,
			Strategy:  []string{"sliding-window"},
			Threshold: 80,
		},
	})

	a := agent.New("test",
		agent.WithModel(llm.Model{
			ID: "test-model",
			Config: &llm.ModelConfig{
				MaxInputTokens: 1000,
			},
		}),
	)

	msgs := make([]llm.Message, 0, 20)
	msgs = append(msgs, llm.SystemMessage("system prompt"))
	for i := 0; i < 9; i++ {
		msgs = append(msgs, llm.UserMessage("user msg"))
		msgs = append(msgs, llm.AssistantMessage("assistant msg"))
	}

	origLen := len(msgs)
	ref := &msgs
	ctx := agent.WithAgent(context.Background(), a)
	ctx = agent.WithHistory(ctx, ref)
	req := &llm.Request{Messages: msgs}

	m.mu.Lock()
	m.needsCompact = true
	m.lastPromptTokens = 900
	m.mu.Unlock()

	_ = m.BeforeLLM(ctx, req)

	if len(req.Messages) >= origLen {
		t.Errorf("context-window sliding window should reduce messages: got %d, original %d", len(req.Messages), origLen)
	}

	if req.Messages[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %v", req.Messages[0].Role)
	}

	if !strings.Contains(llm.MessageText(req.Messages[1]), "compacted") {
		t.Error("second message should be compaction notice")
	}
}

func TestBeforeLLMEmitsStreamAndBusEvents(t *testing.T) {
	m := New(Deps{
		Config: &config.CompactionConfig{
			Enabled:     true,
			Strategy:    []string{"sliding-window"},
			MaxMessages: 3,
		},
	})

	msgs := make([]llm.Message, 0, 10)
	msgs = append(msgs, llm.SystemMessage("system prompt"))
	for i := 0; i < 8; i++ {
		msgs = append(msgs, llm.UserMessage("user msg"))
		msgs = append(msgs, llm.AssistantMessage("assistant msg"))
	}

	ref := &msgs

	a := agent.New("agent-1",
		agent.WithModel(llm.Model{
			ID: "m1",
			Config: &llm.ModelConfig{
				MaxInputTokens: 1000,
			},
		}),
	)
	a.Bus = event.New()

	var streamEvt *agent.StreamEvent
	stream := func(ev agent.StreamEvent) error {
		streamEvt = &ev
		return nil
	}

	ctx := agent.WithAgent(context.Background(), a)
	ctx = agent.WithHistory(ctx, ref)
	ctx = agent.WithStream(ctx, stream)

	var busEvt *Event
	_ = a.Bus.Subscribe(TopicCompactionFinished, func(e Event) { busEvt = &e })

	req := &llm.Request{Messages: msgs}
	m.mu.Lock()
	m.needsCompact = true
	m.lastPromptTokens = 900
	m.mu.Unlock()

	if err := m.BeforeLLM(ctx, req); err != nil {
		t.Fatal(err)
	}

	if busEvt == nil {
		t.Fatal("bus event not published")
	}
	if busEvt.AgentID == "" || busEvt.AgentName != "agent-1" || busEvt.Strategy != "sliding-window" {
		t.Fatalf("bus event = %+v", busEvt)
	}
	if len(busEvt.CompactedMessages) == 0 {
		t.Fatal("bus event missing compacted messages")
	}

	if streamEvt == nil {
		t.Fatal("stream event not emitted")
	}
	if streamEvt.Type != agent.StreamCompaction {
		t.Fatalf("stream type = %q, want %q", streamEvt.Type, agent.StreamCompaction)
	}
	if streamEvt.AgentID == "" || streamEvt.AgentName != "agent-1" || streamEvt.Strategy != "sliding-window" || streamEvt.Messages == 0 || streamEvt.Tokens != 900 {
		t.Fatalf("stream event = %+v", streamEvt)
	}
}

type summaryProvider struct {
	mu    sync.Mutex
	calls int
	fail  bool
}

func (p *summaryProvider) Name() string { return "summary" }

func (p *summaryProvider) Chat(_ context.Context, _ *llm.Request) (*llm.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.fail {
		return nil, errors.New("provider down")
	}
	msg := llm.AssistantMessage("concise conversation summary")
	fr := llm.FinishReasonStop
	return &llm.Response{Choices: []llm.Choice{{Message: &msg, FinishReason: &fr}}}, nil
}

func (p *summaryProvider) ChatStream(context.Context, *llm.Request, llm.StreamHandler) error {
	return nil
}

func (p *summaryProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }

func (p *summaryProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func newSummarizationMiddleware(t *testing.T, cfg *config.CompactionConfig) *Middleware {
	t.Helper()
	mgr := llm.NewManager(event.New(), &logger.Logger{}, nil)
	return New(Deps{Config: cfg, LLM: mgr})
}

func summaryHistory() []llm.Message {
	msgs := []llm.Message{llm.SystemMessage("system prompt")}
	for i := 0; i < 6; i++ {
		msgs = append(msgs, llm.UserMessage("question"))
		msgs = append(msgs, llm.AssistantMessage("answer"))
	}
	msgs = append(msgs, llm.UserMessage("latest question"))
	return msgs
}

func TestCompactSummarizationShrinksHistory(t *testing.T) {
	m := newSummarizationMiddleware(t, &config.CompactionConfig{
		Enabled:   true,
		Strategy:  []string{"summarization"},
		Threshold: 80,
	})
	prov := &summaryProvider{}
	parent := agent.New("parent", agent.WithProvider(prov), agent.WithModel(llm.Model{ID: "m1"}))

	history := summaryHistory()
	origLen := len(history)
	ref := &history
	ctx := agent.WithAgent(context.Background(), parent)
	ctx = agent.WithHistory(ctx, ref)
	req := &llm.Request{Messages: history}

	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()

	if err := m.BeforeLLM(ctx, req); err != nil {
		t.Fatalf("BeforeLLM: %v", err)
	}
	if len(req.Messages) >= origLen {
		t.Fatalf("history not compacted: %d >= %d", len(req.Messages), origLen)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("compacted history = %d messages, want 3", len(req.Messages))
	}
	if req.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("first message should be system, got %v", req.Messages[0].Role)
	}
	if !strings.Contains(llm.MessageText(req.Messages[1]), "concise conversation summary") {
		t.Fatalf("summary message = %q", llm.MessageText(req.Messages[1]))
	}
	if llm.MessageText(req.Messages[2]) != "latest question" {
		t.Fatalf("last user message not preserved: %q", llm.MessageText(req.Messages[2]))
	}
	if prov.Calls() != 1 {
		t.Fatalf("summarizer calls = %d, want 1", prov.Calls())
	}
}

func TestCompactSummarizationKeepsHistoryOnFailure(t *testing.T) {
	m := newSummarizationMiddleware(t, &config.CompactionConfig{
		Enabled:   true,
		Strategy:  []string{"summarization"},
		Threshold: 80,
	})
	prov := &summaryProvider{fail: true}
	parent := agent.New("parent", agent.WithProvider(prov), agent.WithModel(llm.Model{ID: "m1"}))

	history := summaryHistory()
	ref := &history
	ctx := agent.WithAgent(context.Background(), parent)
	ctx = agent.WithHistory(ctx, ref)
	req := &llm.Request{Messages: history}

	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()

	if err := m.BeforeLLM(ctx, req); err != nil {
		t.Fatalf("BeforeLLM should not surface summarizer errors: %v", err)
	}
	if len(req.Messages) != len(history) {
		t.Fatalf("history changed after a failed summary: %d != %d", len(req.Messages), len(history))
	}

	prov.mu.Lock()
	prov.fail = false
	prov.mu.Unlock()
	m.mu.Lock()
	m.needsCompact = true
	m.mu.Unlock()
	if err := m.BeforeLLM(ctx, req); err != nil {
		t.Fatalf("BeforeLLM retry: %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("retry did not compact: %d messages", len(req.Messages))
	}
}

func TestAfterLLMUsesFallbackContextWindow(t *testing.T) {
	m := New(Deps{Config: &config.CompactionConfig{
		Enabled:   true,
		Strategy:  []string{"sliding-window"},
		Threshold: 80,
	}})
	a := agent.New("test", agent.WithModel(llm.Model{ID: "no-config"}))
	ctx := agent.WithAgent(context.Background(), a)

	resp := &llm.Response{Usage: llm.Usage{PromptTokens: DefaultContextWindow}}
	if err := m.AfterLLM(ctx, &llm.Request{}, resp); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	needs := m.needsCompact
	m.mu.Unlock()
	if !needs {
		t.Fatal("compaction should trigger at the fallback window even without model config")
	}
}
