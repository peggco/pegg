package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func (c *Chat) taskCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	fore := th.Base().Foreground(th.Foreground).Background(th.Background)
	success := th.Base().Foreground(th.Success).Background(th.Background)
	errStyle := th.Base().Foreground(th.Error).Background(th.Background)
	runStyle := th.Base().Foreground(lerpColor(th.Running, th.Accent, glowT(c.now))).Bold(true).Background(th.Background)

	status := it.ToolTaskStatus
	if status == "" {
		status = "running"
	}

	duration := i18n.T("chat.status.finished")
	if it.Duration > 0 {
		duration = formatDuration(it.Duration)
	}

	var statusText string
	var statusStyle tcell.Style
	switch status {
	case "running":
		statusText = " " + string(spinnerAt(c.now)) + " " + i18n.T("chat.status.running")
		statusStyle = runStyle
	case "error":
		duration = i18n.T("chat.status.error")
		if it.Duration > 0 {
			duration = formatDuration(it.Duration)
		}
		statusText = " ✖ " + duration
		statusStyle = errStyle.Bold(true)
	case "finished":
		statusText = " ✓ " + duration
		statusStyle = success.Bold(true)
	case "background":
		statusStyle = muted.Bold(true)
	default:
		statusStyle = muted
	}

	textCells := func(s string, st tcell.Style) Line {
		var l Line
		for _, r := range s {
			l = append(l, Cell{R: r, S: st})
		}
		return l
	}

	top := Line{{R: '╭', S: border}, {R: '─', S: border}}
	for _, r := range i18n.T("chat.card.task") {
		top = append(top, Cell{R: r, S: accent})
	}
	label := it.ToolTaskName
	if label == "" {
		label = it.ToolName
	}
	for _, r := range label {
		top = append(top, Cell{R: r, S: fore.Bold(true)})
	}
	labelW := top.Width()
	if it.ToolTaskType != "" {
		for _, r := range " · " + it.ToolTaskType {
			top = append(top, Cell{R: r, S: muted})
		}
	}
	if it.ToolTaskBackground {
		for _, r := range " · " + i18n.T("chat.task.background") {
			top = append(top, Cell{R: r, S: muted})
		}
	}

	maxTop := width - 1 - DisplayWidth(statusText)
	if maxTop < labelW+2 {
		statusText = ""
		maxTop = width - 2
	}
	if top.Width() > maxTop {
		top = TruncateLine(top, maxTop)
	}
	if statusText != "" {
		for top.Width() < width-1-DisplayWidth(statusText) {
			top = append(top, Cell{R: ' ', S: th.Base()})
		}
		for _, r := range statusText {
			top = append(top, Cell{R: r, S: statusStyle})
		}
	}
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines := []Line{top}

	row := func(cells Line) Line {
		l := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		maxContent := width - 3
		if maxContent < 1 {
			maxContent = 1
		}
		if len(cells) > 0 {
			if cells.Width() > maxContent {
				cells = TruncateLine(cells, maxContent)
			}
			l = append(l, cells...)
		}
		for l.Width() < width-1 {
			l = append(l, Cell{R: ' ', S: th.Base()})
		}
		l = append(l, Cell{R: '│', S: border})
		return l
	}

	truncated := false

	if p := strings.TrimSpace(it.ToolTaskPrompt); p != "" {
		promptLines := WrapText(p, fore, width-4)
		const maxPromptLines = 4
		if !it.Expanded && len(promptLines) > maxPromptLines {
			truncated = true
			remaining := len(promptLines) - maxPromptLines
			promptLines = promptLines[:maxPromptLines]
			for _, ln := range promptLines {
				lines = append(lines, row(ln))
			}
			lines = append(lines, row(textCells(i18n.Tf("chat.tool.moreLines", remaining), dim)))
		} else {
			for _, ln := range promptLines {
				lines = append(lines, row(ln))
			}
		}
	}

	lines = append(lines, cardSep(border, width))

	switch status {
	case "running":
		if it.Activity != "" {
			act := Line{{R: spinnerAt(c.now), S: runStyle}, {R: ' ', S: th.Base()}}
			act = append(act, textCells(it.Activity, dim)...)
			lines = append(lines, row(act))
		} else {
			lines = append(lines, row(textCells(i18n.T("chat.task.starting"), dim)))
		}
	case "finished":
		out := strings.TrimSpace(it.ToolOutput)
		if out == "" {
			out = i18n.T("chat.bash.noOutput")
		}
		rawLines := strings.Split(out, "\n")
		const maxTaskOutputLines = 15
		hasMore := len(rawLines) > maxTaskOutputLines
		shown := rawLines
		if hasMore && !it.Expanded {
			shown = rawLines[:maxTaskOutputLines]
			truncated = true
		}
		for _, ln := range shown {
			lines = append(lines, row(textCells(ln, dim)))
		}
		if truncated {
			lines = append(lines, row(textCells(i18n.Tf("chat.tool.moreLines", len(rawLines)-maxTaskOutputLines), dim)))
		}
		if it.Expanded && hasMore {
			lines = append(lines, row(textCells(i18n.Tf("chat.tool.showingAll", len(rawLines)), dim)))
		}
	case "error":
		errLines := WrapText(it.ToolErr, errStyle, width-4)
		const maxTaskErrorLines = 15
		hasMore := len(errLines) > maxTaskErrorLines
		shown := errLines
		if hasMore && !it.Expanded {
			shown = errLines[:maxTaskErrorLines]
			truncated = true
		}
		for _, ln := range shown {
			lines = append(lines, row(ln))
		}
		if truncated {
			lines = append(lines, row(textCells(i18n.Tf("chat.tool.moreLines", len(errLines)-maxTaskErrorLines), dim)))
		}
		if it.Expanded && hasMore {
			lines = append(lines, row(textCells(i18n.Tf("chat.tool.showingAll", len(errLines)), dim)))
		}
	case "background":
		lines = append(lines, row(textCells(i18n.T("chat.task.backgroundNote"), dim)))
	default:
		lines = append(lines, row(textCells(i18n.T("chat.task.noResult"), dim)))
	}

	footer := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	var leftCells Line
	if it.ToolTaskUsage.TotalTokens > 0 {
		leftCells = textCells(llm.FormatContextUsage(it.ToolTaskUsage, 0), accent)
	}
	right := ""
	if status == "running" && it.ToolTaskAgentID != "" {
		right = i18n.T("chat.history")
	} else if truncated {
		right = i18n.T("common.expand")
		if it.Expanded {
			right = i18n.T("common.collapse")
		}
	}
	avail := width - 3
	if avail < 0 {
		avail = 0
	}
	if DisplayWidth(right) > avail {
		right = ""
	}
	if right != "" && leftCells.Width()+DisplayWidth(right) > avail {
		leftCells = TruncateLine(leftCells, max(avail-DisplayWidth(right), 0))
	}
	footer = append(footer, leftCells...)
	if right != "" {
		for footer.Width() < width-1-DisplayWidth(right) {
			footer = append(footer, Cell{R: ' ', S: th.Base()})
		}
		for _, r := range right {
			footer = append(footer, Cell{R: r, S: dim})
		}
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
	}
	footer = append(footer, Cell{R: '│', S: border})
	if len(leftCells) > 0 || right != "" {
		lines = append(lines, cardSep(border, width))
		lines = append(lines, footer)
	}
	lines = append(lines, bottomEdge(border, width))

	return lines
}

func (c *Chat) subagentLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	subagentColor := th.Base().Foreground(th.Subagent).Background(th.Background)

	status := it.SubagentStatus
	if status == "" {
		status = "running"
	}
	isRunning := status == "running"

	var mark rune
	if isRunning {
		mark = '◌'
	} else if status == "error" {
		mark = '✖'
	} else {
		mark = '✔'
	}

	var lines []Line

	top := Line{{R: '╭', S: border}}
	top = append(top, Cell{R: '─', S: border})
	top = append(top, Cell{R: ' ', S: th.Base()})
	top = append(top, Cell{R: mark, S: subagentColor})
	top = append(top, Cell{R: ' ', S: th.Base()})
	historyHint := i18n.T("chat.history")
	maxName := width - 13
	if maxName < 1 {
		maxName = 1
	}
	name := it.SubagentName
	if DisplayWidth(name) > maxName {
		name = truncateToDisplayWidth(name, maxName-1) + "…"
	}
	for _, r := range name {
		top = append(top, Cell{R: r, S: subagentColor})
	}
	historyStart := width - 1 - DisplayWidth(historyHint)
	for top.Width() < historyStart {
		top = append(top, Cell{R: '─', S: border})
	}
	for _, r := range historyHint {
		top = append(top, Cell{R: r, S: muted})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	task := it.SubagentTask
	if task == "" {
		task = "…"
	}
	taskLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	if DisplayWidth(task) > width-6 {
		task = truncateToDisplayWidth(task, width-7) + "…"
	}
	for _, r := range task {
		taskLine = append(taskLine, Cell{R: r, S: th.Base().Foreground(th.Foreground).Background(th.Background)})
	}
	for taskLine.Width() < width-1 {
		taskLine = append(taskLine, Cell{R: ' ', S: th.Base()})
	}
	taskLine = append(taskLine, Cell{R: '│', S: border})
	lines = append(lines, taskLine)

	sep := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep = append(sep, Cell{R: '─', S: border})
	}
	sep = append(sep, Cell{R: '┤', S: border})
	lines = append(lines, sep)

	if it.SubagentActivity != "" && isRunning {
		activityLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		activityMark := '◌'
		activityLine = append(activityLine, Cell{R: activityMark, S: th.Base().Foreground(lerpColor(th.Subagent, th.Accent, glowT(c.now))).Background(th.Background)})
		activityLine = append(activityLine, Cell{R: ' ', S: th.Base()})
		activity := it.SubagentActivity
		if DisplayWidth(activity) > width-7 {
			activity = truncateToDisplayWidth(activity, width-8) + "…"
		}
		for _, r := range activity {
			activityLine = append(activityLine, Cell{R: r, S: dim})
		}
		for activityLine.Width() < width-1 {
			activityLine = append(activityLine, Cell{R: ' ', S: th.Base()})
		}
		activityLine = append(activityLine, Cell{R: '│', S: border})
		lines = append(lines, activityLine)
	} else if it.SubagentOutput != "" && !isRunning {
		outputLines := strings.Split(it.SubagentOutput, "\n")
		const maxOutputLines = 15
		hasMore := len(outputLines) > maxOutputLines
		truncated := hasMore && !it.Expanded
		shown := outputLines
		if truncated {
			shown = outputLines[:maxOutputLines]
		}
		for _, ln := range shown {
			row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
			if DisplayWidth(ln) > width-4 {
				ln = truncateToDisplayWidth(ln, width-5) + "…"
			}
			for _, r := range ln {
				row = append(row, Cell{R: r, S: dim})
			}
			for row.Width() < width-1 {
				row = append(row, Cell{R: ' ', S: th.Base()})
			}
			row = append(row, Cell{R: '│', S: border})
			lines = append(lines, row)
		}
		if truncated {
			remaining := len(outputLines) - maxOutputLines
			moreLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
			msg := i18n.Tf("chat.tool.moreLines", remaining)
			for _, r := range msg {
				moreLine = append(moreLine, Cell{R: r, S: dim})
			}
			for moreLine.Width() < width-1 {
				moreLine = append(moreLine, Cell{R: ' ', S: th.Base()})
			}
			moreLine = append(moreLine, Cell{R: '│', S: border})
			lines = append(lines, moreLine)
		}
		if it.Expanded && hasMore {
			moreLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
			msg := i18n.Tf("chat.tool.showingAll", len(outputLines))
			for _, r := range msg {
				moreLine = append(moreLine, Cell{R: r, S: dim})
			}
			for moreLine.Width() < width-1 {
				moreLine = append(moreLine, Cell{R: ' ', S: th.Base()})
			}
			moreLine = append(moreLine, Cell{R: '│', S: border})
			lines = append(lines, moreLine)
		}
	}

	sep2 := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep2 = append(sep2, Cell{R: '─', S: border})
	}
	sep2 = append(sep2, Cell{R: '┤', S: border})
	lines = append(lines, sep2)

	footer := Line{{R: '╰', S: border}}
	usageText := ""
	if it.SubagentUsage.TotalTokens > 0 {
		usageText = llm.FormatContextUsage(it.SubagentUsage, 0)
	}
	if usageText != "" {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
		for _, r := range usageText {
			footer = append(footer, Cell{R: r, S: accent})
		}
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: '─', S: border})
	}
	footer = append(footer, Cell{R: '╯', S: border})
	lines = append(lines, footer)

	return lines
}
