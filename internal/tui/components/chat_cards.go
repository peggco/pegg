package components

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func cardEdge(left, right rune, width int, border tcell.Style) Line {
	line := Line{{R: left, S: border}}
	for line.Width() < width-1 {
		line = append(line, Cell{R: '─', S: border})
	}
	line = append(line, Cell{R: right, S: border})
	return line
}

func cardRow(ln Line, width int, border tcell.Style, th styles.Theme) Line {
	bg := th.Base().Foreground(th.Foreground).Background(th.UserBg)
	row := Line{{R: '│', S: border}, {R: ' ', S: bg}}
	row = append(row, ln...)
	for row.Width() < width-1 {
		row = append(row, Cell{R: ' ', S: bg})
	}
	row = append(row, Cell{R: '│', S: border})
	return row
}

func cardSep(border tcell.Style, width int) Line {
	sep := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep = append(sep, Cell{R: '─', S: border})
	}
	sep = append(sep, Cell{R: '┤', S: border})
	return sep
}

func cardFooter(border tcell.Style, dim tcell.Style, bg tcell.Style, width int, text string) Line {
	footer := Line{{R: '│', S: border}, {R: ' ', S: bg}}
	for _, r := range text {
		footer = append(footer, Cell{R: r, S: dim})
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: bg})
	}
	footer = append(footer, Cell{R: '│', S: border})
	return footer
}

func bottomEdge(border tcell.Style, width int) Line {
	bottom := Line{{R: '╰', S: border}}
	for i := 0; i < width-2; i++ {
		bottom = append(bottom, Cell{R: '─', S: border})
	}
	bottom = append(bottom, Cell{R: '╯', S: border})
	return bottom
}

func attachmentChipLine(a llm.Attachment, width int, th styles.Theme) Line {
	var icon rune
	switch a.Type {
	case llm.AttachmentTypeImage:
		icon = '🖼'
	case llm.AttachmentTypeAudio:
		icon = '🎵'
	default:
		icon = '📎'
	}
	bg := th.Base().Foreground(th.Foreground).Background(th.UserBg)
	nameStyle := th.Base().Foreground(th.TextDim).Background(th.UserBg)
	line := Line{
		{R: ' ', S: bg},
		{R: icon, S: bg},
		{R: ' ', S: bg},
	}
	name := a.FileName
	for line.Width()+DisplayWidth(name)+4 > width {
		if len(name) <= 1 {
			break
		}
		name = name[:len(name)-1]
	}
	if a.FileName != name {
		name += "…"
	}
	for _, r := range name {
		line = append(line, Cell{R: r, S: nameStyle})
	}
	return line
}

func (c *Chat) userCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 8 {
		return WrapText(it.Text, th.Base().Foreground(th.InputText).Background(th.UserBg), width)
	}
	border := th.Base().Foreground(th.Border).Background(th.UserBg)

	var lines []Line
	lines = append(lines, cardEdge('┌', '┐', width, border))

	content := WrapText(it.Text, th.Base().Foreground(th.Foreground).Background(th.UserBg), width-4)
	if len(content) == 0 {
		content = []Line{{}}
	}
	for _, ln := range content {
		lines = append(lines, cardRow(ln, width, border, th))
	}
	for _, a := range it.Attachments {
		lines = append(lines, cardRow(attachmentChipLine(a, width-4, th), width, border, th))
	}

	lines = append(lines, cardEdge('└', '┘', width, border))
	return lines
}

func (c *Chat) thinkingLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)

	active := c.isActiveThinking(it)
	var mark rune
	var markStyle tcell.Style
	if active {
		mark = '◌'
		markStyle = th.Base().Foreground(th.ThinkingGlow).Background(th.Background)
	} else {
		mark = '▸'
		if it.Expanded {
			mark = '▾'
		}
		markStyle = th.Base().Foreground(th.Accent).Background(th.Background)
	}

	label := i18n.T("chat.thinking")
	labelStyle := th.Base().Foreground(th.TextDim).Background(th.Background)
	if active {
		labelStyle = th.Base().Foreground(lerpColor(th.ThinkingDim, th.ThinkingGlow, glowT(c.now))).Background(th.Background)
	}

	line := Line{
		{R: mark, S: markStyle},
		{R: ' ', S: dim},
	}
	for _, r := range label {
		line = append(line, Cell{R: r, S: labelStyle})
	}

	if active {
		dots := int(c.now / (300 * time.Millisecond) % 4)
		for i := 0; i < 4; i++ {
			r := '·'
			if i >= dots {
				r = ' '
			}
			line = append(line, Cell{R: r, S: dim})
		}
	}

	var lines []Line
	lines = append(lines, line)

	if it.Expanded && it.Reasoning != "" {
		reasonStyle := th.Base().Foreground(th.Reasoning).Background(th.Background)
		body := WrapText(it.Reasoning, reasonStyle, width)
		lines = append(lines, body...)
	}
	return lines
}

func (c *Chat) isActiveThinking(it *ChatItem) bool {
	for i, item := range c.items {
		if item == it {
			for j := i + 1; j < len(c.items); j++ {
				if c.items[j].Kind != ItemThinking {
					return false
				}
			}
			return true
		}
	}
	return false
}

func fitCardText(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if DisplayWidth(s) <= max {
		return s
	}
	return truncateToDisplayWidth(s, max-1) + "\u2026"
}
