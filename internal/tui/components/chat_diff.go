package components

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func (c *Chat) editCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	filePath := it.ToolName
	if idx := strings.Index(filePath, ":"); idx > 0 {
		filePath = filePath[idx+1:]
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	fore := th.Base().Foreground(th.Foreground).Background(th.Background)
	success := th.Base().Foreground(th.Success).Background(th.Background)

	addBg := lerpColor(th.Success, th.Background, 0.82)
	delBg := lerpColor(th.Error, th.Background, 0.82)
	addFill := th.Base().Background(addBg)
	delFill := th.Base().Background(delBg)
	addMark := th.Base().Foreground(th.Success).Bold(true).Background(addBg)
	delMark := th.Base().Foreground(th.Error).Bold(true).Background(delBg)
	addNum := th.Base().Foreground(th.Muted).Background(addBg)
	delNum := th.Base().Foreground(th.Muted).Background(delBg)
	addBase := th.Base().Foreground(th.CodeText).Background(addBg)
	delBase := th.Base().Foreground(th.CodeText).Background(delBg)
	ctxBase := th.Base().Foreground(th.TextDim).Background(th.Background)

	maxLine := 1
	for _, h := range it.Diff {
		if v := h.OldStart + h.OldCount; v > maxLine {
			maxLine = v
		}
		if v := h.NewStart + h.NewCount; v > maxLine {
			maxLine = v
		}
	}
	numW := len(strconv.Itoa(maxLine))
	if numW > 6 {
		numW = 6
	}
	if maxW := width - 7; numW > maxW {
		numW = maxW
	}
	if numW < 1 {
		numW = 1
	}

	numCell := func(n int, st tcell.Style) Line {
		s := strconv.Itoa(n)
		if len(s) > numW {
			s = s[len(s)-numW:]
		}
		var out Line
		for i := len(s); i < numW; i++ {
			out = append(out, Cell{R: ' ', S: st})
		}
		for _, r := range s {
			out = append(out, Cell{R: r, S: st})
		}
		return out
	}

	var lines []Line

	top := Line{{R: '╭', S: border}, {R: '─', S: border}}
	for _, r := range i18n.T("chat.card.diff") {
		top = append(top, Cell{R: r, S: accent})
	}

	statusText := " ✓ " + formatDuration(it.Duration)
	statusStyle := success.Bold(true)

	pathLabel := filePath
	needed := top.Width() + DisplayWidth(pathLabel) + 2 + DisplayWidth(statusText)
	for needed > width-2 && len(pathLabel) > 4 {
		pathLabel = pathLabel[:len(pathLabel)-1]
		needed = top.Width() + DisplayWidth(pathLabel) + 2 + DisplayWidth(statusText)
	}
	for _, r := range pathLabel {
		top = append(top, Cell{R: r, S: fore})
	}
	topPad := width - 2 - top.Width() - DisplayWidth(statusText)
	if topPad < 1 {
		topPad = 1
	}
	for i := 0; i < topPad; i++ {
		top = append(top, Cell{R: ' ', S: th.Base()})
	}
	for _, r := range statusText {
		top = append(top, Cell{R: r, S: statusStyle})
	}
	top = append(top, Cell{R: ' ', S: th.Base()})
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	lines = append(lines, cardSep(border, width))

	borderedText := func(text string, style tcell.Style) Line {
		row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		for _, r := range text {
			if row.Width() >= width-2 {
				break
			}
			row = append(row, Cell{R: r, S: style})
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: th.Base()})
		}
		row = append(row, Cell{R: '│', S: border})
		return row
	}

	emit := func(dl DiffLine) {
		var num, mark, content Line
		var fill tcell.Style
		switch dl.Kind {
		case '-':
			num = numCell(dl.OldLine, delNum)
			mark = Line{{R: '-', S: delMark}}
			content = highlightLine(dl.Text, filePath, th, delBase)
			fill = delFill
		case '+':
			num = numCell(dl.NewLine, addNum)
			mark = Line{{R: '+', S: addMark}}
			content = highlightLine(dl.Text, filePath, th, addBase)
			fill = addFill
		default:
			num = numCell(dl.OldLine, muted)
			mark = Line{{R: ' ', S: th.Base()}}
			content = highlightLine(dl.Text, filePath, th, ctxBase)
			fill = th.Base().Background(th.Background)
		}

		row := Line{{R: '│', S: border}, {R: ' ', S: fill}}
		row = append(row, num...)
		row = append(row, Cell{R: ' ', S: fill})
		row = append(row, mark...)
		row = append(row, Cell{R: ' ', S: fill})
		for _, cell := range content {
			if row.Width() >= width-1 {
				break
			}
			row = append(row, cell)
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: fill})
		}
		row = append(row, Cell{R: '│', S: border})
		lines = append(lines, row)
	}

	totalRows, addedCount, removedCount := 0, 0, 0
	for _, h := range it.Diff {
		totalRows += len(h.Lines)
		for _, dl := range h.Lines {
			switch dl.Kind {
			case '-':
				removedCount++
			case '+':
				addedCount++
			}
		}
	}

	const maxDiffRows = 30
	hasMore := totalRows > maxDiffRows
	truncated := hasMore && !it.Expanded
	emitted, skipped := 0, 0

	for _, h := range it.Diff {
		for _, dl := range h.Lines {
			if truncated && emitted >= maxDiffRows {
				skipped++
				continue
			}
			emit(dl)
			emitted++
		}
	}

	if skipped > 0 {
		lines = append(lines, borderedText(i18n.Tf("chat.tool.moreLines", skipped), muted))
	}
	if it.Expanded && hasMore {
		lines = append(lines, borderedText(i18n.Tf("chat.tool.showingAll", totalRows), muted))
	}

	lines = append(lines, cardSep(border, width))

	addStyle := th.Base().Foreground(th.Success).Bold(true).Background(th.Background)
	delStyle := th.Base().Foreground(th.Error).Bold(true).Background(th.Background)
	footer := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	footer = append(footer, Cell{R: '+', S: addStyle})
	for _, r := range strconv.Itoa(addedCount) {
		footer = append(footer, Cell{R: r, S: addStyle})
	}
	footer = append(footer, Cell{R: ' ', S: th.Base()})
	footer = append(footer, Cell{R: '-', S: delStyle})
	for _, r := range strconv.Itoa(removedCount) {
		footer = append(footer, Cell{R: r, S: delStyle})
	}

	if hasMore {
		expandHint := i18n.T("common.expand")
		if it.Expanded {
			expandHint = i18n.T("common.collapse")
		}
		for footer.Width() < width-1-DisplayWidth(expandHint) {
			footer = append(footer, Cell{R: ' ', S: th.Base()})
		}
		for _, r := range expandHint {
			footer = append(footer, Cell{R: r, S: muted})
		}
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
	}
	footer = append(footer, Cell{R: '│', S: border})
	lines = append(lines, footer)

	lines = append(lines, bottomEdge(border, width))

	return lines
}

func (c *Chat) writeCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	filePath := it.ToolName
	if idx := strings.Index(filePath, ":"); idx > 0 {
		filePath = filePath[idx+1:]
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	fore := th.Base().Foreground(th.Foreground).Background(th.Background)
	success := th.Base().Foreground(th.Success).Background(th.Background)
	fill := th.Base().Background(th.Background)
	ctxBase := th.Base().Foreground(th.TextDim).Background(th.Background)

	content := it.WriteContent
	if content == "" {
		content = i18n.T("chat.write.empty")
	}

	rawLines := strings.Split(content, "\n")
	const maxWriteLines = 30
	hasMore := len(rawLines) > maxWriteLines
	truncated := hasMore && !it.Expanded
	shown := rawLines
	if truncated {
		shown = rawLines[:maxWriteLines]
	}

	numW := len(strconv.Itoa(len(rawLines)))
	if numW > 6 {
		numW = 6
	}
	if maxW := width - 7; numW > maxW {
		numW = maxW
	}
	if numW < 1 {
		numW = 1
	}

	numCell := func(n int) Line {
		s := strconv.Itoa(n)
		if len(s) > numW {
			s = s[len(s)-numW:]
		}
		var out Line
		for i := len(s); i < numW; i++ {
			out = append(out, Cell{R: ' ', S: muted})
		}
		for _, r := range s {
			out = append(out, Cell{R: r, S: muted})
		}
		return out
	}

	var lines []Line

	top := Line{{R: '╭', S: border}, {R: '─', S: border}}
	for _, r := range i18n.T("chat.card.write") {
		top = append(top, Cell{R: r, S: accent})
	}

	statusText := " ✓ " + formatDuration(it.Duration)
	statusStyle := success.Bold(true)

	pathLabel := filePath
	needed := top.Width() + DisplayWidth(pathLabel) + 2 + DisplayWidth(statusText)
	for needed > width-2 && len(pathLabel) > 4 {
		pathLabel = pathLabel[:len(pathLabel)-1]
		needed = top.Width() + DisplayWidth(pathLabel) + 2 + DisplayWidth(statusText)
	}
	for _, r := range pathLabel {
		top = append(top, Cell{R: r, S: fore})
	}
	topPad := width - 2 - top.Width() - DisplayWidth(statusText)
	if topPad < 1 {
		topPad = 1
	}
	for i := 0; i < topPad; i++ {
		top = append(top, Cell{R: ' ', S: th.Base()})
	}
	for _, r := range statusText {
		top = append(top, Cell{R: r, S: statusStyle})
	}
	top = append(top, Cell{R: ' ', S: th.Base()})
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	lines = append(lines, cardSep(border, width))

	borderedText := func(text string) Line {
		row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		for _, r := range text {
			if row.Width() >= width-2 {
				break
			}
			row = append(row, Cell{R: r, S: muted})
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: th.Base()})
		}
		row = append(row, Cell{R: '│', S: border})
		return row
	}

	for i, ln := range shown {
		if strings.IndexByte(ln, '\t') >= 0 {
			ln = expandDiffTabs(ln)
		}
		row := Line{{R: '│', S: border}, {R: ' ', S: fill}}
		row = append(row, numCell(i+1)...)
		row = append(row, Cell{R: ' ', S: fill}, Cell{R: ' ', S: fill}, Cell{R: ' ', S: fill})
		for _, cell := range highlightLine(ln, filePath, th, ctxBase) {
			if row.Width() >= width-1 {
				break
			}
			row = append(row, cell)
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: fill})
		}
		row = append(row, Cell{R: '│', S: border})
		lines = append(lines, row)
	}

	if truncated {
		remaining := len(rawLines) - maxWriteLines
		lines = append(lines, borderedText(i18n.Tf("chat.tool.moreLines", remaining)))
	}

	if it.Expanded && hasMore {
		lines = append(lines, borderedText(i18n.Tf("chat.tool.showingAll", len(rawLines))))
	}

	lines = append(lines, cardSep(border, width))

	footer := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	for _, r := range i18n.Tf("chat.write.lines", len(rawLines)) {
		footer = append(footer, Cell{R: r, S: muted})
	}
	if hasMore {
		expandHint := i18n.T("common.expand")
		if it.Expanded {
			expandHint = i18n.T("common.collapse")
		}
		for footer.Width() < width-1-DisplayWidth(expandHint) {
			footer = append(footer, Cell{R: ' ', S: th.Base()})
		}
		for _, r := range expandHint {
			footer = append(footer, Cell{R: r, S: muted})
		}
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
	}
	footer = append(footer, Cell{R: '│', S: border})
	lines = append(lines, footer)

	lines = append(lines, bottomEdge(border, width))

	return lines
}
