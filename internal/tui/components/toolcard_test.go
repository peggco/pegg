package components

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/tui/styles"
)

func flattenLines(lines []Line) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		for _, c := range l {
			b.WriteRune(c.R)
		}
	}
	return b.String()
}

func TestDefaultToolHasNoExpand(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{Kind: ItemTool, ToolName: "read:foo.go", ToolOutput: "line1\nline2\nline3"}
	lines := c.toolLines(it, 80)
	if len(lines) != 1 {
		t.Fatalf("default tool should render one line, got %d", len(lines))
	}
	if strings.Contains(flattenLines(lines), "Expand") {
		t.Error("default tool with output should not offer expand")
	}
}

func TestDefaultToolErrorShowsExpandAndError(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{Kind: ItemTool, ToolName: "read:foo.go", ToolErr: "permission denied", ToolOutput: "raw output"}
	lines := c.toolLines(it, 80)
	if !strings.Contains(flattenLines(lines), "Expand") {
		t.Error("errored tool should offer expand")
	}

	it.Expanded = true
	text := flattenLines(c.toolLines(it, 80))
	if !strings.Contains(text, "permission denied") {
		t.Error("expanded error should show the error text")
	}
	if strings.Contains(text, "raw output") {
		t.Error("expanded error must not show tool output")
	}
}

func TestCardsHideExpandWhenComplete(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	write := &ChatItem{Kind: ItemTool, ToolName: "write:a.go", WriteContent: "a\nb\n", ToolOutput: "ok"}
	if strings.Contains(flattenLines(c.writeCardLines(write, 80)), "Expand") {
		t.Error("complete write card should not offer expand")
	}

	edit := &ChatItem{Kind: ItemTool, ToolName: "edit:a.go", Diff: ComputeDiff("a\nb\n", "a\nc\n"), ToolOutput: "ok"}
	if strings.Contains(flattenLines(c.editCardLines(edit, 80)), "Expand") {
		t.Error("complete diff card should not offer expand")
	}

	bash := &ChatItem{Kind: ItemTool, ToolName: "bash:echo hi", ToolOutput: "hi", Duration: 0}
	if strings.Contains(flattenLines(c.bashCardLines(bash, 80)), "Expand") {
		t.Error("complete bash card should not offer expand")
	}
}

func TestCardsShowExpandWhenTruncated(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString("line\n")
	}

	write := &ChatItem{Kind: ItemTool, ToolName: "write:a.go", WriteContent: sb.String(), ToolOutput: "ok"}
	if !strings.Contains(flattenLines(c.writeCardLines(write, 80)), "Expand") {
		t.Error("truncated write card should offer expand")
	}

	edit := &ChatItem{Kind: ItemTool, ToolName: "edit:a.go", Diff: ComputeDiff("", sb.String()), ToolOutput: "ok"}
	if !strings.Contains(flattenLines(c.editCardLines(edit, 80)), "Expand") {
		t.Error("truncated diff card should offer expand")
	}

	bash := &ChatItem{Kind: ItemTool, ToolName: "bash:seq 40", ToolOutput: sb.String()}
	if !strings.Contains(flattenLines(c.bashCardLines(bash, 80)), "Expand") {
		t.Error("truncated bash card should offer expand")
	}
}
