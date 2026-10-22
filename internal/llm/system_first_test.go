package llm

import (
	"context"
	"strings"
	"testing"
)

func TestFoldLateSystemMessagesKeepsLeadingPrompt(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		UserMessage("hello"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded != nil {
		t.Fatalf("expected no change, got %+v", folded)
	}
}

func TestFoldLateSystemMessagesFoldsIntoPreviousToolMessage(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		UserMessage("do it"),
		AssistantMessage("working"),
		ToolMessage("tool output", "call-1"),
		SystemMessage("<system-reminder>\nsubagent finished\n</system-reminder>"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded == nil {
		t.Fatal("expected the late system message to be folded")
	}
	if len(folded) != 4 {
		t.Fatalf("message count = %d, want 4", len(folded))
	}
	for i, msg := range folded {
		if msg.Role == RoleSystem && i > 0 {
			t.Fatalf("message %d is still a system message", i)
		}
	}
	if folded[0].Role != RoleSystem {
		t.Fatalf("leading role = %q, want system", folded[0].Role)
	}
	tool := folded[3]
	if tool.Role != RoleTool {
		t.Fatalf("tool role = %q", tool.Role)
	}
	if !strings.Contains(MessageText(tool), "tool output") ||
		!strings.Contains(MessageText(tool), "subagent finished") {
		t.Fatalf("tool content = %q", MessageText(tool))
	}
}

func TestFoldLateSystemMessagesFoldsIntoNextUserMessage(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		SystemMessage("<system-reminder>\nmemory panel\n</system-reminder>"),
		UserMessage("what changed?"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded == nil {
		t.Fatal("expected the late system message to be folded")
	}
	if len(folded) != 2 {
		t.Fatalf("message count = %d, want 2", len(folded))
	}
	if folded[0].Role != RoleSystem {
		t.Fatalf("first role = %q, want system", folded[0].Role)
	}
	user := folded[1]
	if user.Role != RoleUser {
		t.Fatalf("second role = %q, want user", user.Role)
	}
	if !strings.Contains(MessageText(user), "what changed?") ||
		!strings.Contains(MessageText(user), "memory panel") {
		t.Fatalf("user content = %q", MessageText(user))
	}
}

func TestFoldLateSystemMessagesConvertsOrphanSystemMessage(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		AssistantMessage("already answered"),
		SystemMessage("context compacted"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded == nil {
		t.Fatal("expected the late system message to be folded")
	}
	if len(folded) != 3 {
		t.Fatalf("message count = %d, want 3", len(folded))
	}
	last := folded[2]
	if last.Role != RoleUser {
		t.Fatalf("role = %q, want user", last.Role)
	}
	if !strings.Contains(MessageText(last), "context compacted") {
		t.Fatalf("content = %q", MessageText(last))
	}
}

func TestFoldLateSystemMessagesMovesSystemPromptToFront(t *testing.T) {
	messages := []Message{
		UserMessage("earlier turn"),
		SystemMessage("you are pegg"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded == nil {
		t.Fatal("expected a rewrite")
	}
	if folded[0].Role != RoleSystem {
		t.Fatalf("first role = %q, want system", folded[0].Role)
	}
	if MessageText(folded[0]) != "you are pegg" {
		t.Fatalf("system content = %q", MessageText(folded[0]))
	}
}

func TestFoldLateSystemMessagesKeepsOriginalSliceIntact(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		UserMessage("hello"),
		SystemMessage("late"),
	}
	before := MessageText(messages[1])
	FoldLateSystemMessages(messages)
	if MessageText(messages[1]) != before {
		t.Fatalf("original message was modified: %q", MessageText(messages[1]))
	}
	if messages[2].Role != RoleSystem {
		t.Fatalf("original system message was modified: %q", messages[2].Role)
	}
}

func TestFoldLateSystemMessagesHandlesContentParts(t *testing.T) {
	messages := []Message{
		SystemMessage("you are pegg"),
		Message{
			Role:    RoleUser,
			Content: []any{map[string]any{"type": "text", "text": "hello"}},
		},
		SystemMessage("late"),
	}
	folded := FoldLateSystemMessages(messages)
	if folded == nil {
		t.Fatal("expected a rewrite")
	}
	parts, ok := folded[1].Content.([]any)
	if !ok {
		t.Fatalf("content type = %T", folded[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("part count = %d, want 2", len(parts))
	}
	last, ok := parts[1].(map[string]any)
	if !ok || last["text"] != "late" {
		t.Fatalf("last part = %+v", parts[1])
	}
}

type systemFirstRecorder struct {
	messages []Message
}

func (p *systemFirstRecorder) Name() string { return "recording" }

func (p *systemFirstRecorder) Chat(_ context.Context, req *Request) (*Response, error) {
	p.messages = req.Messages
	return &Response{}, nil
}

func (p *systemFirstRecorder) ChatStream(_ context.Context, req *Request, _ StreamHandler) error {
	p.messages = req.Messages
	return nil
}

func (p *systemFirstRecorder) ListModels(_ context.Context) ([]Model, error) { return nil, nil }

type systemFirstSkipCache struct {
	systemFirstRecorder
}

func (p *systemFirstSkipCache) SkipModelCache() bool { return true }

func TestSystemFirstProviderFoldsOutgoingRequest(t *testing.T) {
	inner := &systemFirstRecorder{}
	provider := &systemFirstProvider{inner: inner}

	req := NewRequest("model", []Message{
		SystemMessage("you are pegg"),
		UserMessage("hello"),
		SystemMessage("late reminder"),
	})
	if _, err := provider.Chat(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(inner.messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(inner.messages))
	}
	for i, msg := range inner.messages {
		if msg.Role == RoleSystem && i > 0 {
			t.Fatalf("message %d is a late system message", i)
		}
	}
	if len(req.Messages) != 3 {
		t.Fatalf("original request was modified: %d messages", len(req.Messages))
	}
}

func TestSystemFirstProviderForwardsSkipModelCache(t *testing.T) {
	cached := &systemFirstProvider{inner: &systemFirstSkipCache{}}
	if !cached.SkipModelCache() {
		t.Fatal("SkipModelCache must be forwarded")
	}
	plain := &systemFirstProvider{inner: &systemFirstRecorder{}}
	if plain.SkipModelCache() {
		t.Fatal("SkipModelCache must be false when the inner provider does not implement it")
	}
}
