package components

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/llm"
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

func TestSendFileCardListsFiles(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{
		Kind:     ItemTool,
		ToolName: "sendfile:2 files",
		ToolDone: true,
		Attachments: []llm.Attachment{
			{Type: llm.AttachmentTypeFile, FileName: "report.pdf", Path: "/tmp/report.pdf", Size: 1536},
			{Type: llm.AttachmentTypeFile, FileName: "app.zip", Path: "/tmp/app.zip", Size: 5 << 20},
		},
	}
	lines := c.toolLines(it, 80)
	out := flattenLines(lines)
	for _, want := range []string{"report.pdf", "1.5 KB", "/tmp/report.pdf", "app.zip", "5.0 MB"} {
		if !strings.Contains(out, want) {
			t.Fatalf("sendfile card missing %q:\n%s", want, out)
		}
	}
}

func TestSendFileCardSpacingBetweenBoxes(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{
		Kind:     ItemTool,
		ToolName: "sendfile:2 files",
		ToolDone: true,
		Attachments: []llm.Attachment{
			{Type: llm.AttachmentTypeFile, FileName: "report.pdf", Path: "/tmp/report.pdf", Size: 1536},
			{Type: llm.AttachmentTypeFile, FileName: "app.zip", Path: "/tmp/app.zip", Size: 5 << 20},
		},
	}
	lines := c.toolLines(it, 60)
	if len(lines) != 8 {
		t.Fatalf("lines = %d, want 8", len(lines))
	}
	boxes := c.fileBoxesFor(it, 60)
	if len(boxes) != 2 {
		t.Fatalf("boxes = %d, want 2", len(boxes))
	}
	if boxes[0].start != 1 || boxes[0].end != 4 {
		t.Fatalf("first box = %+v, want lines 1-3", boxes[0])
	}
	if boxes[1].start != 5 || boxes[1].end != 8 {
		t.Fatalf("second box = %+v, want lines 5-7", boxes[1])
	}
	if boxes[0].path != "/tmp/report.pdf" || boxes[1].path != "/tmp/app.zip" {
		t.Fatalf("box paths = %q, %q", boxes[0].path, boxes[1].path)
	}
}

func TestSendFileCardRendersClickableBoxes(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{
		Kind:     ItemTool,
		ToolName: "sendfile:1 file",
		ToolDone: true,
		Attachments: []llm.Attachment{
			{Type: llm.AttachmentTypeFile, FileName: "report.pdf", Path: "/tmp/report.pdf", Size: 1536},
		},
	}
	out := flattenLines(c.toolLines(it, 60))
	for _, want := range []string{"┌", "│", "└", "report.pdf", "1.5 KB", "/tmp/report.pdf"} {
		if !strings.Contains(out, want) {
			t.Fatalf("boxed card missing %q:\n%s", want, out)
		}
	}
	boxes := c.fileBoxesFor(it, 60)
	if len(boxes) != 1 {
		t.Fatalf("boxes = %d, want 1", len(boxes))
	}
	if boxes[0].path != "/tmp/report.pdf" || boxes[0].end-boxes[0].start != 3 {
		t.Fatalf("box = %+v, want 3 lines for /tmp/report.pdf", boxes[0])
	}
}

func TestSendFileBoxClickOpensFile(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	item := &ChatItem{
		Kind:     ItemTool,
		ToolName: "sendfile:1 file",
		ToolDone: true,
		Attachments: []llm.Attachment{
			{Type: llm.AttachmentTypeFile, FileName: "report.pdf", Path: "/tmp/report.pdf", Size: 1536},
		},
	}
	c.AppendItem(item)

	var opened string
	var activated *ChatItem
	c.SetOnOpenFile(func(path string) { opened = path })
	c.SetOnActivate(func(it *ChatItem) { activated = it })
	c.rebuildFlat(60)

	if !c.HandleClick(5, 2, 0) {
		t.Fatal("click should be handled")
	}
	if opened != "/tmp/report.pdf" {
		t.Fatalf("opened = %q, want /tmp/report.pdf", opened)
	}
	if activated != nil {
		t.Fatal("clicking a file box must not activate the item")
	}

	c.HandleClick(5, 0, 0)
	if activated != item {
		t.Fatal("clicking the card header should activate the item")
	}
}

func TestSendFileCardFallsBackToChipsWhenNarrow(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	c := NewChat()

	it := &ChatItem{
		Kind:     ItemTool,
		ToolName: "sendfile:1 file",
		ToolDone: true,
		Attachments: []llm.Attachment{
			{Type: llm.AttachmentTypeFile, FileName: "report.pdf", Path: "/tmp/report.pdf", Size: 1536},
		},
	}
	out := flattenLines(c.toolLines(it, 20))
	if strings.Contains(out, "┌") || !strings.Contains(out, "report.pdf") {
		t.Fatalf("narrow card should use a chip line:\n%s", out)
	}
	boxes := c.fileBoxesFor(it, 20)
	if len(boxes) != 1 || boxes[0].end-boxes[0].start != 1 {
		t.Fatalf("chip box = %+v, want a single clickable line", boxes)
	}
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
