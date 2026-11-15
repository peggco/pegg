package components

import (
	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func ViewStyle(name string, base tcell.Style) tcell.Style {
	th := styles.Current()
	switch name {
	case "muted":
		return base.Foreground(th.Muted)
	case "hint":
		return base.Foreground(th.Hint)
	case "accent":
		return base.Foreground(th.Accent)
	case "error":
		return base.Foreground(th.Error)
	case "warning":
		return base.Foreground(th.Warning)
	case "success":
		return base.Foreground(th.Success)
	case "bold":
		return base.Bold(true)
	default:
		return base
	}
}

func ViewLineSpans(line pkgplugin.ViewLine) []pkgplugin.ViewSpan {
	if len(line.Spans) > 0 {
		return line.Spans
	}
	if line.Text != "" {
		return []pkgplugin.ViewSpan{{Text: line.Text}}
	}
	return nil
}

func ViewLineWidth(line pkgplugin.ViewLine) int {
	width := 0
	for _, span := range ViewLineSpans(line) {
		width += DisplayWidth(span.Text)
	}
	return width
}

func DrawViewLine(s tcell.Screen, x, y, maxWidth int, line pkgplugin.ViewLine, base tcell.Style) int {
	used := 0
	for _, span := range ViewLineSpans(line) {
		if maxWidth > 0 && used >= maxWidth {
			break
		}
		text := span.Text
		if maxWidth > 0 {
			text = TruncateTo(text, maxWidth-used)
		}
		DrawText(s, x+used, y, text, ViewStyle(span.Style, base))
		used += DisplayWidth(text)
	}
	return used
}

func DrawViewLines(s tcell.Screen, x, y, maxWidth, maxHeight int, lines []pkgplugin.ViewLine, base tcell.Style) int {
	drawn := 0
	for _, line := range lines {
		if maxHeight > 0 && drawn >= maxHeight {
			break
		}
		DrawViewLine(s, x, y+drawn, maxWidth, line, base)
		drawn++
	}
	return drawn
}

func ViewFragmentsHeight(fragments []pkgplugin.ViewFragment) int {
	height := 0
	for _, fragment := range fragments {
		if fragment.Title != "" {
			height++
		}
		height += len(fragment.Lines)
	}
	return height
}

func DrawViewFragment(s tcell.Screen, x, y, maxWidth int, fragment pkgplugin.ViewFragment, base tcell.Style) int {
	line := 0
	if fragment.Title != "" {
		title := fragment.Title
		if fragment.Badge != "" {
			title = "[" + fragment.Badge + "] " + title
		}
		DrawText(s, x, y, TruncateTo(title, maxWidth), ViewStyle("accent", base).Bold(true))
		line++
	}
	line += DrawViewLines(s, x, y+line, maxWidth, len(fragment.Lines), fragment.Lines, base)
	return line
}
