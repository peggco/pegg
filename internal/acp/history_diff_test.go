package acp

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func historyToolCallSend(t *testing.T, sends []string) string {
	t.Helper()
	for _, s := range sends {
		if strings.Contains(s, `"tool_call"`) {
			return s
		}
	}
	t.Fatalf("no tool_call update: %v", sends)
	return ""
}

func TestHistoryReplayIncludesEditDiff(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt

	msgs := []session.Message{
		{
			ID: "a1", Seq: 1, Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID: "c1", Type: "function",
				Function: llm.Function{
					Name:      "edit",
					Arguments: `{"filePath":"internal/x.go","oldString":"old","newString":"new"}`,
				},
			}},
		},
		{ID: "t1", Seq: 2, Role: llm.RoleTool, ToolCallID: "c1", Content: "ok"},
	}

	srv.emitHistoryMessages("sess", "/tmp/project", msgs, false)

	bt.mu.Lock()
	sends := append([]string(nil), bt.sends...)
	bt.mu.Unlock()
	call := historyToolCallSend(t, sends)
	for _, want := range []string{`"type":"diff"`, `"oldText":"old"`, `"newText":"new"`, `"path":"internal/x.go"`} {
		if !strings.Contains(call, want) {
			t.Fatalf("tool_call missing %q:\n%s", want, call)
		}
	}
}

func TestHistoryReplayIncludesWriteDiff(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt

	msgs := []session.Message{
		{
			ID: "a1", Seq: 1, Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID: "c1", Type: "function",
				Function: llm.Function{
					Name:      "write",
					Arguments: `{"filePath":"internal/y.go","content":"package y\n"}`,
				},
			}},
		},
	}

	srv.emitHistoryMessages("sess", "/tmp/project", msgs, false)

	bt.mu.Lock()
	sends := append([]string(nil), bt.sends...)
	bt.mu.Unlock()
	call := historyToolCallSend(t, sends)
	for _, want := range []string{`"type":"diff"`, `"path":"internal/y.go"`, `"newText":"package y\n"`} {
		if !strings.Contains(call, want) {
			t.Fatalf("tool_call missing %q:\n%s", want, call)
		}
	}
	if strings.Contains(call, `"oldText"`) {
		t.Fatalf("write diff should not include oldText:\n%s", call)
	}
}
