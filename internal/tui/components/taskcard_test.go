package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/styles"
)

func taskCardBase() *ChatItem {
	return &ChatItem{
		Kind:           ItemTool,
		ToolName:       "task:explore-api",
		ToolTaskName:   "explore-api",
		ToolTaskType:   "explorer",
		ToolTaskPrompt: "Map the REST API",
	}
}

func TestTaskCardRendersBoxInEveryState(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	cases := []struct {
		name   string
		mutate func(*ChatItem)
		want   string
	}{
		{"running", func(it *ChatItem) { it.ToolTaskStatus = "running" }, "running"},
		{"finished", func(it *ChatItem) {
			it.ToolTaskStatus, it.ToolDone, it.ToolOutput = "finished", true, "answer"
		}, "answer"},
		{"error", func(it *ChatItem) {
			it.ToolTaskStatus, it.ToolDone, it.ToolErr = "error", true, "boom"
		}, "boom"},
		{"background", func(it *ChatItem) {
			it.ToolTaskStatus, it.ToolDone, it.ToolTaskBackground = "background", true, true
		}, "Background subagent"},
		{"unknown", func(it *ChatItem) {
			it.ToolTaskStatus, it.ToolDone = "unknown", true
		}, "no result recorded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it := taskCardBase()
			tc.mutate(it)
			out := flattenLines(c.toolLines(it, 80))
			for _, want := range []string{"╭", "╰", "TASK", "explore-api", "explorer", "Map the REST API", tc.want} {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestTaskCardShowsBackgroundBadge(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()
	it := taskCardBase()
	it.ToolTaskBackground = true
	it.ToolTaskStatus = "background"
	it.ToolDone = true
	out := flattenLines(c.toolLines(it, 80))
	if !strings.Contains(out, "· background") || !strings.Contains(out, "Background subagent") {
		t.Fatalf("expected header badge and body note, got:\n%s", out)
	}
}

func TestTaskCardLinesNeverExceedWidth(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	base := taskCardBase()
	base.ToolTaskName = strings.Repeat("very-long-name-", 6)
	base.ToolTaskType = strings.Repeat("type-", 8)
	base.ToolTaskPrompt = strings.Repeat("prompt line ", 20)
	base.ToolTaskStatus = "finished"
	base.ToolDone = true
	base.ToolOutput = strings.Repeat("output line\n", 30)
	base.ToolTaskUsage = llm.Usage{TotalTokens: 1234, PromptTokens: 1000, CompletionTokens: 234}

	for _, width := range []int{12, 20, 40, 80, 96} {
		for _, line := range c.toolLines(base, width) {
			if line.Width() > width {
				t.Fatalf("width %d: line width %d exceeds limit:\n%s", width, line.Width(), flattenLines([]Line{line}))
			}
		}
	}
}

func TestTaskCardHistoryOnlyWhileRunning(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()
	it := taskCardBase()
	it.ToolTaskAgentID = "sub-1"
	it.ToolTaskStatus = "running"
	c.SetItems([]*ChatItem{it})
	c.rebuildFlat(80)

	if _, ok := c.historyButtons[0]; !ok {
		t.Fatalf("running task card should register a history target: %v", c.historyButtons)
	}

	it.ToolTaskStatus = "finished"
	it.ToolDone = true
	it.ToolOutput = "answer"
	c.InvalidateItemPtr(it)
	c.rebuildFlat(80)
	if _, ok := c.historyButtons[0]; ok {
		t.Fatalf("finished task card should not register a history target: %v", c.historyButtons)
	}
}

func TestTaskCardExpandRevealsTruncatedOutput(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()
	it := taskCardBase()
	it.ToolTaskStatus = "finished"
	it.ToolDone = true
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "line-%02d\n", i)
	}
	it.ToolOutput = b.String()

	collapsed := flattenLines(c.toolLines(it, 80))
	if !strings.Contains(collapsed, "more lines") {
		t.Fatalf("collapsed task output should advertise more lines:\n%s", collapsed)
	}
	if strings.Contains(collapsed, "line-20") {
		t.Fatalf("collapsed task output should not include line-20:\n%s", collapsed)
	}

	it.Expanded = true
	expanded := flattenLines(c.toolLines(it, 80))
	if !strings.Contains(expanded, "line-20") {
		t.Fatalf("expanded task output should include line-20:\n%s", expanded)
	}
	if !strings.Contains(expanded, "showing all") {
		t.Fatalf("expanded task output should show the showing-all hint:\n%s", expanded)
	}
}

func TestTaskCardErrorExpandShowsError(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()
	it := taskCardBase()
	it.ToolTaskStatus = "error"
	it.ToolDone = true
	it.ToolErr = strings.Repeat("failure detail ", 200)

	collapsed := flattenLines(c.toolLines(it, 80))
	if !strings.Contains(collapsed, "more lines") {
		t.Fatalf("collapsed error should advertise more lines:\n%s", collapsed)
	}
	if !strings.Contains(collapsed, "failure detail") {
		t.Fatalf("collapsed error should show the error:\n%s", collapsed)
	}
}
