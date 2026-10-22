package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent/reminder"
	"github.com/peggco/pegg/internal/llm"
)

func TestSystemFirstOnlyFoldsLateSystemMessages(t *testing.T) {
	base, prov := newTestAgent(t, WithSystemPrompt("SYSTEM PROMPT"), WithTool(echoTool()))
	prov.responses = []mockResponse{
		{calls: []llm.ToolCall{toolCall("c1", "echo", "hi")}},
		{content: "done"},
	}
	rec := &recordingProvider{Provider: base.Provider}
	base.Provider = llm.WithSystemFirstOnly(rec)
	base.QueueNotification(reminder.New("subagent", "SUBAGENT FINISHED"))

	if _, err := base.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if len(rec.reqs) == 0 {
		t.Fatal("no request recorded")
	}
	for i, req := range rec.reqs {
		for j, msg := range req.Messages {
			if msg.Role == llm.RoleSystem && j > 0 {
				t.Fatalf("req[%d] message %d is a late system message: %+v", i, j, msg)
			}
		}
		if req.Messages[0].Role != llm.RoleSystem {
			t.Fatalf("req[%d] first message role = %q, want system", i, req.Messages[0].Role)
		}
		if !strings.Contains(fmt.Sprint(req.Messages[0].Content), "SYSTEM PROMPT") {
			t.Fatalf("req[%d] system prompt = %+v", i, req.Messages[0].Content)
		}
	}
	found := false
	for _, req := range rec.reqs {
		for _, msg := range req.Messages {
			if strings.Contains(fmt.Sprint(msg.Content), "SUBAGENT FINISHED") {
				found = true
				if msg.Role == llm.RoleSystem {
					t.Fatalf("notification must not stay a system message: %+v", msg)
				}
			}
		}
	}
	if !found {
		t.Fatal("queued notification was not delivered")
	}
}
