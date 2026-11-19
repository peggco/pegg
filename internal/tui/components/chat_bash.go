package components

import (
	"strings"

	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func (c *Chat) bashCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	cmd := it.ToolName
	if idx := strings.Index(cmd, ":"); idx > 0 {
		cmd = cmd[idx+1:]
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)
	muted := th.Base().Foreground(th.Muted).Background(th.Background)
	success := th.Base().Foreground(th.Success).Background(th.Background)
	running := !it.ToolDone && it.ToolErr == ""

	var lines []Line

	top := Line{{R: '╭', S: border}}
	top = append(top, Cell{R: '─', S: border})
	for _, r := range i18n.T("chat.card.bash") {
		top = append(top, Cell{R: r, S: accent})
	}

	cmdStyle := th.Base().Foreground(th.Foreground).Background(th.Background)
	statusText := " ✓ " + formatDuration(it.Duration)
	statusStyle := success.Bold(true)
	if running {
		statusText = " " + string(spinnerAt(c.now)) + " " + i18n.T("chat.status.running")
		statusStyle = th.Base().Foreground(lerpColor(th.Running, th.Accent, glowT(c.now))).Bold(true).Background(th.Background)
	}

	cmdLabel := "$ " + cmd
	rightSide := statusText
	needed := top.Width() + DisplayWidth(cmdLabel) + 2 + DisplayWidth(rightSide)
	for needed > width-2 {
		if len(cmdLabel) > 4 {
			cmdLabel = cmdLabel[:len(cmdLabel)-1]
		} else {
			break
		}
		needed = top.Width() + DisplayWidth(cmdLabel) + 2 + DisplayWidth(rightSide)
	}

	for _, r := range cmdLabel {
		top = append(top, Cell{R: r, S: cmdStyle})
	}
	topPad := width - 2 - top.Width() - DisplayWidth(rightSide)
	if topPad < 1 {
		topPad = 1
	}
	for i := 0; i < topPad; i++ {
		top = append(top, Cell{R: ' ', S: th.Base()})
	}
	for _, r := range rightSide {
		top = append(top, Cell{R: r, S: statusStyle})
	}
	top = append(top, Cell{R: ' ', S: th.Base()})
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	sep := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep = append(sep, Cell{R: '─', S: border})
	}
	sep = append(sep, Cell{R: '┤', S: border})
	lines = append(lines, sep)

	if running {
		if partial := strings.TrimRight(it.ToolPartial, "\n"); partial != "" {
			partialLines := strings.Split(partial, "\n")
			const maxPartialLines = 8
			if len(partialLines) > maxPartialLines {
				partialLines = partialLines[len(partialLines)-maxPartialLines:]
			}
			for _, line := range partialLines {
				prow := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
				for _, r := range line {
					if prow.Width() >= width-2 {
						break
					}
					prow = append(prow, Cell{R: r, S: dim})
				}
				for prow.Width() < width-1 {
					prow = append(prow, Cell{R: ' ', S: th.Base()})
				}
				prow = append(prow, Cell{R: '│', S: border})
				lines = append(lines, prow)
			}
		}
		lines = append(lines, bottomEdge(border, width))
		return lines
	}

	output := it.ToolOutput
	if output == "" {
		output = i18n.T("chat.bash.noOutput")
	}

	rawLines := strings.Split(output, "\n")
	const maxBashLines = 30
	hasMore := len(rawLines) > maxBashLines
	truncated := hasMore && !it.Expanded
	shown := rawLines
	if truncated {
		shown = rawLines[:maxBashLines]
	}

	bodyStyle := th.Base().Foreground(th.Foreground).Background(th.Background)
	for _, ln := range shown {
		row := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		hl := highlightByLangName(ln, "bash", th, bodyStyle)
		for _, c := range hl {
			if row.Width() >= width-2 {
				break
			}
			row = append(row, c)
		}
		for row.Width() < width-1 {
			row = append(row, Cell{R: ' ', S: th.Base()})
		}
		row = append(row, Cell{R: '│', S: border})
		lines = append(lines, row)
	}

	if truncated {
		remaining := len(rawLines) - maxBashLines
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
		msg := i18n.Tf("chat.tool.showingAll", len(rawLines))
		for _, r := range msg {
			moreLine = append(moreLine, Cell{R: r, S: dim})
		}
		for moreLine.Width() < width-1 {
			moreLine = append(moreLine, Cell{R: ' ', S: th.Base()})
		}
		moreLine = append(moreLine, Cell{R: '│', S: border})
		lines = append(lines, moreLine)
	}

	sep2 := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep2 = append(sep2, Cell{R: '─', S: border})
	}
	sep2 = append(sep2, Cell{R: '┤', S: border})
	lines = append(lines, sep2)

	footer := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	exitStr := i18n.Tf("chat.bash.exitCode", 0)
	if it.ToolErr != "" {
		exitStr = i18n.Tf("chat.bash.exitCode", 1)
	}
	for _, r := range exitStr {
		footer = append(footer, Cell{R: r, S: muted})
	}

	expandHint := i18n.T("common.expand")
	if it.Expanded {
		expandHint = i18n.T("common.collapse")
	}
	if hasMore {
		for footer.Width() < width-1-DisplayWidth(expandHint) {
			footer = append(footer, Cell{R: ' ', S: th.Base()})
		}
		for _, r := range expandHint {
			footer = append(footer, Cell{R: r, S: dim})
		}
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
	}
	footer = append(footer, Cell{R: '│', S: border})
	lines = append(lines, footer)

	bottom := Line{{R: '╰', S: border}}
	for i := 0; i < width-2; i++ {
		bottom = append(bottom, Cell{R: '─', S: border})
	}
	bottom = append(bottom, Cell{R: '╯', S: border})
	lines = append(lines, bottom)

	return lines
}
