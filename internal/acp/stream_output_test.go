package acp

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent"
)

func TestHandleStreamEventToolOutput(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt

	st := &promptState{
		sessionID:  "sess-1",
		textIDs:    make(map[string]string),
		thoughtID:  make(map[string]string),
		oldFiles:   make(map[string]string),
		toolOutput: make(map[string]*strings.Builder),
	}

	for _, chunk := range []string{"hello\n", "world\n"} {
		srv.handleStreamEvent(st, &ACPSession{}, agent.StreamEvent{
			Type:       agent.StreamToolOutput,
			ToolCallID: "call-1",
			ToolName:   "bash",
			ToolStream: "stdout",
			ToolChunk:  chunk,
		})
	}

	if bt.count() != 2 {
		t.Fatalf("sends = %d, want 2", bt.count())
	}
	last := bt.last()
	for _, want := range []string{`"tool_call_update"`, `"in_progress"`, "hello", "world"} {
		if !strings.Contains(last, want) {
			t.Fatalf("update missing %q:\n%s", want, last)
		}
	}
}
