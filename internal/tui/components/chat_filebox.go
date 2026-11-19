package components

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

type fileBox struct {
	path       string
	start, end int
}

const minFileBoxWidth = 24

func (c *Chat) sendFileCardLines(it *ChatItem, width int) []Line {
	lines, _ := c.sendFileCard(it, width)
	return lines
}

func (c *Chat) fileBoxesFor(it *ChatItem, width int) []fileBox {
	if it.Kind != ItemTool || len(it.Attachments) == 0 {
		return nil
	}
	if it.ToolName != "sendfile" && !strings.HasPrefix(it.ToolName, "sendfile:") {
		return nil
	}
	if it.ToolErr != "" || (!it.ToolDone && it.ToolOutput == "") {
		return nil
	}
	_, boxes := c.sendFileCard(it, width)
	return boxes
}

func (c *Chat) sendFileCard(it *ChatItem, width int) ([]Line, []fileBox) {
	th := styles.Current()
	if width < 12 || len(it.Attachments) == 0 {
		return nil, nil
	}
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	fore := th.Base().Foreground(th.Foreground).Background(th.Background)
	success := th.Base().Foreground(th.Success).Background(th.Background)

	lines := []Line{LineFromSegments([]Segment{
		{Text: "✔ ", Style: success},
		{Text: i18n.Tf("chat.files.sent", len(it.Attachments)), Style: fore.Bold(true)},
		{Text: "  " + i18n.T("chat.files.copyHint"), Style: muted},
	}, width)}

	boxes := make([]fileBox, 0, len(it.Attachments))
	for i, a := range it.Attachments {
		if i > 0 && width >= minFileBoxWidth {
			lines = append(lines, Line{})
		}
		start := len(lines)
		if width >= minFileBoxWidth {
			lines = append(lines, fileBoxLines(a, width, th)...)
		} else {
			lines = append(lines, fileChipLine(a, width, th))
		}
		boxes = append(boxes, fileBox{path: a.Path, start: start, end: len(lines)})
	}
	return lines, boxes
}

func fileDisplayName(a llm.Attachment) string {
	name := a.FileName
	if name == "" && a.Path != "" {
		name = filepath.Base(a.Path)
	}
	if name == "" {
		name = "file"
	}
	return name
}

func fileChipLine(a llm.Attachment, width int, th styles.Theme) Line {
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	row := []Segment{
		{Text: "  📎 ", Style: muted},
		{Text: fileDisplayName(a), Style: accent},
	}
	if size := formatFileSize(a.Size); size != "" {
		row = append(row, Segment{Text: "  " + size, Style: muted})
	}
	return LineFromSegments(row, width)
}

func fileBoxLines(a llm.Attachment, width int, th styles.Theme) []Line {
	border := th.Base().Foreground(th.Border).Background(th.Background)
	nameStyle := th.Base().Foreground(th.Accent).Bold(true).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)

	name := fileDisplayName(a)
	size := formatFileSize(a.Size)
	suffix := " ┐"
	if size != "" {
		suffix = " " + size + " ┐"
	}

	maxName := width - 5 - 1 - DisplayWidth(suffix) - 1
	if maxName < 3 {
		maxName = 3
	}
	name = truncateToDisplayWidth(name, maxName)
	fill := width - 5 - DisplayWidth(name) - 1 - DisplayWidth(suffix)
	if fill < 1 {
		fill = 1
	}

	top := LineFromSegments([]Segment{
		{Text: "┌ 📎 ", Style: border},
		{Text: name, Style: nameStyle},
		{Text: " ", Style: border},
		{Text: strings.Repeat("─", fill), Style: border},
		{Text: suffix, Style: muted},
	}, width)

	inner := width - 4
	if inner < 1 {
		inner = 1
	}
	path := a.Path
	if path == "" {
		path = name
	}
	path = truncateToDisplayWidth(path, inner)
	pad := inner - DisplayWidth(path)
	if pad < 0 {
		pad = 0
	}
	body := LineFromSegments([]Segment{
		{Text: "│ ", Style: border},
		{Text: path, Style: muted},
		{Text: strings.Repeat(" ", pad) + " │", Style: border},
	}, width)

	bottom := LineFromSegments([]Segment{
		{Text: "└" + strings.Repeat("─", max(width-2, 1)) + "┘", Style: border},
	}, width)

	return []Line{top, body, bottom}
}

func formatFileSize(size int64) string {
	if size <= 0 {
		return ""
	}
	const unit = 1024
	if size < unit {
		return strconv.FormatInt(size, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(size)/float64(div), 'f', 1, 64) + " " + string("KMGTP"[exp]) + "B"
}
