package session

import (
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func toolMessage(calls ...llm.ToolCall) Message {
	return Message{Role: llm.RoleAssistant, ToolCalls: calls}
}

func toolCall(name, args string) llm.ToolCall {
	return llm.ToolCall{Function: llm.Function{Name: name, Arguments: args}}
}

func TestSummarizeRevertEditAndWrite(t *testing.T) {
	messages := []Message{
		toolMessage(toolCall("edit", `{"filePath":"src/a.go","oldString":"one\ntwo","newString":"one\nthree"}`)),
		toolMessage(toolCall("write", `{"filePath":"src/b.go","content":"alpha\nbeta"}`)),
		toolMessage(toolCall("read", `{"filePath":"src/c.go"}`)),
	}
	summary := SummarizeRevert(messages)

	if len(summary.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(summary.Files))
	}
	if summary.Files[0].Path != "src/a.go" || summary.Files[0].Additions != 1 || summary.Files[0].Deletions != 1 {
		t.Fatalf("a.go = %+v", summary.Files[0])
	}
	if summary.Files[1].Path != "src/b.go" || summary.Files[1].Additions != 2 || summary.Files[1].Deletions != 0 {
		t.Fatalf("b.go = %+v", summary.Files[1])
	}
	if summary.TotalAdditions != 3 || summary.TotalDeletions != 1 {
		t.Fatalf("totals = +%d -%d", summary.TotalAdditions, summary.TotalDeletions)
	}
	if summary.Files[0].Status != RevertStatusReverted {
		t.Fatalf("status = %q", summary.Files[0].Status)
	}
}

func TestSummarizeRevertMergesRepeatedPaths(t *testing.T) {
	messages := []Message{
		toolMessage(toolCall("edit", `{"file_path":"a.ts","old_string":"a","new_string":"a\nb"}`)),
		toolMessage(toolCall("edit", `{"path":"a.ts","oldString":"c","newString":""}`)),
	}
	summary := SummarizeRevert(messages)
	if len(summary.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(summary.Files))
	}
	file := summary.Files[0]
	if file.Additions != 1 || file.Deletions != 1 {
		t.Fatalf("merged = +%d -%d", file.Additions, file.Deletions)
	}
}

func TestSummarizeRevertSkipsNonEditToolsAndBadArgs(t *testing.T) {
	messages := []Message{
		toolMessage(toolCall("bash", `{"command":"ls"}`)),
		toolMessage(toolCall("edit", `not json`)),
		toolMessage(toolCall("edit", `{}`)),
		Message{Role: llm.RoleUser, ToolCalls: []llm.ToolCall{toolCall("edit", `{"path":"u.go","newString":"x"}`)}},
	}
	summary := SummarizeRevert(messages)
	if len(summary.Files) != 0 {
		t.Fatalf("files = %+v, want none", summary.Files)
	}
}

func TestLineChangeCounts(t *testing.T) {
	tests := []struct {
		name      string
		oldText   string
		newText   string
		adds      int
		deletions int
	}{
		{name: "empty old", oldText: "", newText: "a\nb", adds: 2},
		{name: "empty new", oldText: "a\nb", newText: "", deletions: 2},
		{name: "single replacement", oldText: "a\nb\nc", newText: "a\nx\nc", adds: 1, deletions: 1},
		{name: "append", oldText: "a", newText: "a\nb", adds: 1},
		{name: "identical", oldText: "a\nb", newText: "a\nb"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adds, deletions := lineChangeCounts(test.oldText, test.newText)
			if adds != test.adds || deletions != test.deletions {
				t.Fatalf("got +%d -%d, want +%d -%d", adds, deletions, test.adds, test.deletions)
			}
		})
	}
}
