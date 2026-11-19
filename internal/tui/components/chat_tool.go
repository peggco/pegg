package components

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func (c *Chat) toolLines(it *ChatItem, width int) []Line {
	th := styles.Current()

	if it.ToolTaskName != "" {
		return c.taskCardLines(it, width)
	}

	isEdit := strings.HasPrefix(it.ToolName, "edit:") || it.ToolName == "edit"
	isWrite := strings.HasPrefix(it.ToolName, "write:") || it.ToolName == "write"
	isList := strings.Contains(it.ToolName, "todoread") || strings.Contains(it.ToolName, "todowrite")
	isBash := strings.HasPrefix(it.ToolName, "bash:") || it.ToolName == "bash"
	isAsk := it.ToolName == "askuserquestion"
	isSendFile := it.ToolName == "sendfile" || strings.HasPrefix(it.ToolName, "sendfile:")
	isRunning := !it.ToolDone && it.ToolErr == ""

	if isRunning {
		if isBash {
			return c.bashCardLines(it, width)
		}
		return []Line{c.runningToolLine(it, width)}
	}

	if it.ToolErr != "" {
		if it.Expanded {
			left := c.toolStatusLine(it, width, '✖', th.Error, " ✖ "+formatDuration(it.Duration)+"  "+i18n.T("common.collapse"))
			var lines []Line
			lines = append(lines, left)
			bodyStyle := th.Base().Foreground(th.Error).Background(th.Background)
			wrapped := WrapText(it.ToolErr, bodyStyle, width-4)
			for _, ln := range wrapped {
				row := Line{{R: ' ', S: th.Base()}, {R: ' ', S: th.Base()}}
				row = append(row, ln...)
				lines = append(lines, row)
			}
			lines = append(lines, LineFromSegments([]Segment{
				{Text: "  " + i18n.T("common.collapse"), Style: th.Base().Foreground(th.Muted).Background(th.Background)},
			}, width))
			return lines
		}
		left := c.toolStatusLine(it, width, '✖', th.Error, " ✖ "+formatDuration(it.Duration)+"  "+i18n.T("common.expand"))
		return []Line{left}
	}

	if isList {
		return c.todoCardLines(it, width)
	}

	if isBash {
		return c.bashCardLines(it, width)
	}

	if isEdit && len(it.Diff) > 0 {
		return c.editCardLines(it, width)
	}
	if isWrite {
		return c.writeCardLines(it, width)
	}
	if isAsk {
		return c.askCardLines(it, width)
	}
	if isSendFile && len(it.Attachments) > 0 {
		return c.sendFileCardLines(it, width)
	}

	return []Line{c.toolStatusLine(it, width, '✔', th.Success, " ✔ "+formatDuration(it.Duration))}
}

func (c *Chat) runningToolLine(it *ChatItem, width int) Line {
	th := styles.Current()
	label := it.Activity
	if label == "" {
		label = it.ToolName
	}
	pulse := lerpColor(th.TextDim, th.Accent, glowT(c.now))
	markStyle := th.Base().Foreground(pulse).Bold(true).Background(th.Background)
	labelStyle := th.Base().Foreground(th.Foreground).Background(th.Background)
	dimStyle := th.Base().Foreground(th.Muted).Background(th.Background)

	line := Line{{R: spinnerAt(c.now), S: markStyle}, {R: ' ', S: th.Base()}}
	for _, r := range label {
		if line.Width() >= width-4 {
			line = append(line, Cell{R: '…', S: dimStyle})
			break
		}
		line = append(line, Cell{R: r, S: labelStyle})
	}
	dots := int(c.now/(300*time.Millisecond)) % 4
	for i := 0; i < 3; i++ {
		if line.Width() >= width-1 {
			break
		}
		r := ' '
		if i < dots {
			r = '·'
		}
		line = append(line, Cell{R: r, S: dimStyle})
	}
	return line
}

func (c *Chat) toolStatusLine(it *ChatItem, width int, mark rune, color tcell.Color, statusText string) Line {
	th := styles.Current()
	left := Line{
		{R: mark, S: th.Base().Foreground(color).Background(th.Background)},
		{R: ' ', S: th.Base()},
	}
	displayName := it.ToolName
	toolType := ""
	if idx := strings.Index(displayName, ":"); idx > 0 {
		toolType = displayName[:idx]
		displayName = displayName[idx+1:]
	}
	if toolType != "" {
		capType := strings.ToUpper(toolType[:1]) + toolType[1:]
		for _, r := range capType {
			left = append(left, Cell{R: r, S: th.Base().Foreground(color).Bold(true).Background(th.Background)})
		}
		left = append(left, Cell{R: ':', S: th.Base().Foreground(color).Background(th.Background)})
		left = append(left, Cell{R: ' ', S: th.Base()})
	}
	statusCells := LineFromSegments([]Segment{
		{Text: statusText, Style: th.Base().Foreground(color).Background(th.Background)},
	}, width)
	statusW := statusCells.Width()
	for _, r := range displayName {
		if left.Width()+1+statusW >= width {
			left = append(left, Cell{R: '…', S: th.Base().Foreground(th.Muted).Background(th.Background)})
			break
		}
		left = append(left, Cell{R: r, S: th.Base().Foreground(th.Foreground).Bold(true).Background(th.Background)})
	}
	left = append(left, Cell{R: ' ', S: th.Base()})
	maxLeft := width - statusW
	if maxLeft < 0 {
		maxLeft = 0
	}
	if left.Width() > maxLeft {
		left = TruncateLine(left, maxLeft)
	}
	for left.Width() < maxLeft {
		left = append(left, Cell{R: ' ', S: th.Base()})
	}
	left = append(left, statusCells...)
	return left
}

func (c *Chat) todoCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)
	success := th.Base().Foreground(th.Success).Bold(true).Background(th.Background)
	errStyle := th.Base().Foreground(th.Error).Background(th.Background)
	running := th.Base().Foreground(th.Running).Bold(true).Background(th.Background)

	var lines []Line

	top := Line{{R: '╭', S: border}}
	top = append(top, Cell{R: '─', S: border})
	for _, r := range i18n.T("chat.card.todo") {
		top = append(top, Cell{R: r, S: accent})
	}
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	if strings.HasPrefix(strings.TrimSpace(it.ToolOutput), "No todos.") {
		row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		for _, r := range "No todos." {
			row = append(row, Cell{R: r, S: dim})
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: th.Base()})
		}
		row = append(row, Cell{R: '│', S: border})
		lines = append(lines, row)
		sep := cardSep(border, width)
		lines = append(lines, sep)
		lines = append(lines, cardFooter(border, dim, th.Base(), width, i18n.Tf("chat.todo.completed", 0, 0)))
		return lines
	}

	todos := parseTodoItems(it.ToolOutput)

	for _, t := range todos {
		row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}

		var icon rune
		var iconStyle tcell.Style
		var titleStyle tcell.Style

		switch t.status {
		case "completed":
			icon = '✓'
			iconStyle = success
			titleStyle = success
		case "in_progress":
			icon = '◉'
			iconStyle = running
			titleStyle = th.Base().Foreground(th.Foreground).Bold(true).Background(th.Background)
		case "cancelled":
			icon = '✗'
			iconStyle = errStyle
			titleStyle = errStyle
		default:
			icon = '○'
			iconStyle = th.Base().Foreground(th.Muted).Background(th.Background)
			titleStyle = dim
		}

		row = append(row, Cell{R: icon, S: iconStyle}, Cell{R: ' ', S: th.Base()})
		for _, r := range t.title {
			row = append(row, Cell{R: r, S: titleStyle})
		}
		if t.status == "cancelled" {
			for i := len(row) - 2; i >= 0; i-- {
				if row[i].R != ' ' && row[i].R != '│' && row[i].S != th.Base() && row[i].S != border {
					row[i].S = row[i].S.StrikeThrough(true)
				}
			}
		}

		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: th.Base()})
		}
		row = append(row, Cell{R: '│', S: border})
		lines = append(lines, row)

		if t.desc != "" {
			drow := Line{{R: '│', S: border}, Cell{R: ' ', S: th.Base()}, Cell{R: ' ', S: th.Base()}, Cell{R: ' ', S: th.Base()}}
			for _, r := range "    " + t.desc {
				if drow.Width() >= width-2 {
					break
				}
				drow = append(drow, Cell{R: r, S: dim})
			}
			for drow.Width() < width-1 {
				drow = append(drow, Cell{R: ' ', S: th.Base()})
			}
			drow = append(drow, Cell{R: '│', S: border})
			lines = append(lines, drow)
		}
		if t.deps != "" {
			drow := Line{{R: '│', S: border}, Cell{R: ' ', S: th.Base()}, Cell{R: ' ', S: th.Base()}, Cell{R: ' ', S: th.Base()}}
			for _, r := range "    depends on: " + t.deps {
				if drow.Width() >= width-2 {
					break
				}
				drow = append(drow, Cell{R: r, S: dim})
			}
			for drow.Width() < width-1 {
				drow = append(drow, Cell{R: ' ', S: th.Base()})
			}
			drow = append(drow, Cell{R: '│', S: border})
			lines = append(lines, drow)
		}
	}

	total := len(todos)
	completed := 0
	for _, t := range todos {
		if t.status == "completed" {
			completed++
		}
	}

	sep := cardSep(border, width)
	lines = append(lines, sep)
	lines = append(lines, cardFooter(border, dim, th.Base(), width, i18n.Tf("chat.todo.completed", completed, total)))
	lines = append(lines, bottomEdge(border, width))

	return lines
}

type todoItem struct {
	status string
	title  string
	desc   string
	deps   string
}

func parseTodoItems(output string) []todoItem {
	var todos []todoItem
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Todos:") || strings.HasPrefix(trimmed, "No todos.") {
			continue
		}

		status := ""
		rest := trimmed
		switch {
		case strings.HasPrefix(trimmed, "[x]"):
			status = "completed"
			rest = strings.TrimSpace(trimmed[3:])
		case strings.HasPrefix(trimmed, "[~]"):
			status = "in_progress"
			rest = strings.TrimSpace(trimmed[3:])
		case strings.HasPrefix(trimmed, "[-]"):
			status = "cancelled"
			rest = strings.TrimSpace(trimmed[3:])
		case strings.HasPrefix(trimmed, "[ ]"):
			status = "pending"
			rest = strings.TrimSpace(trimmed[3:])
		default:
			if strings.HasPrefix(line, "   ") && len(todos) > 0 {
				cur := &todos[len(todos)-1]
				if strings.HasPrefix(trimmed, "depends on:") {
					cur.deps = strings.TrimSpace(strings.TrimPrefix(trimmed, "depends on:"))
				} else {
					cur.desc = trimmed
				}
			}
			continue
		}

		ti := todoItem{status: status}
		if idx := strings.Index(rest, "]"); idx >= 0 {
			ti.title = strings.TrimSpace(rest[idx+1:])
		} else {
			ti.title = rest
		}
		if pIdx := strings.LastIndex(ti.title, "(priority:"); pIdx >= 0 {
			ti.title = strings.TrimSpace(ti.title[:pIdx])
		}
		todos = append(todos, ti)
	}
	return todos
}
