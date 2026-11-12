package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent"
)

func TestRunRendererStreamsToolOutput(t *testing.T) {
	var buf bytes.Buffer
	r := newRunRenderer(&buf, strings.NewReader(""), "main", false, false)

	r.onToolOutput(agent.AgentToolOutput{
		AgentID:  "main",
		CallID:   "c1",
		ToolName: "bash",
		Chunk:    "hello\n",
	})
	r.onToolResult(agent.AgentToolResult{
		AgentID:  "main",
		CallID:   "c1",
		ToolName: "bash",
		Output:   "Exit code: 0\nstdout:\nhello\nstderr:\n",
	})

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing streamed output:\n%s", out)
	}
	if strings.Contains(out, "stdout:") {
		t.Fatalf("duplicated full output:\n%s", out)
	}
	if !strings.Contains(out, "exit code 0") {
		t.Fatalf("missing exit code:\n%s", out)
	}
}
