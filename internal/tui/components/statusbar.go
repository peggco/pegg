package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type StatusBar struct {
	running         bool
	modelName       string
	provider        string
	reasoningEffort string
	mode            string
	usage           llm.Usage
	maxInputTokens  int
	extraLeft       []pkgplugin.ViewSpan
	extraRight      []pkgplugin.ViewSpan
	replaceLeft     bool
	replaceRight    bool
}

type StatusSnapshot struct {
	Running         bool
	ModelName       string
	Provider        string
	ReasoningEffort string
	Mode            string
	Usage           llm.Usage
	MaxInputTokens  int
}

func NewStatusBar() *StatusBar { return &StatusBar{} }

func (sb *StatusBar) SetRunning(r bool)           { sb.running = r }
func (sb *StatusBar) SetModelName(n string)       { sb.modelName = n }
func (sb *StatusBar) SetProvider(p string)        { sb.provider = p }
func (sb *StatusBar) SetReasoningEffort(e string) { sb.reasoningEffort = e }
func (sb *StatusBar) SetMode(m string)            { sb.mode = m }
func (sb *StatusBar) SetUsage(u llm.Usage)        { sb.usage = u }
func (sb *StatusBar) SetMaxInputTokens(n int)     { sb.maxInputTokens = n }

func (sb *StatusBar) SetViewSpans(left, right []pkgplugin.ViewSpan, replaceLeft, replaceRight bool) {
	sb.extraLeft = left
	sb.extraRight = right
	sb.replaceLeft = replaceLeft
	sb.replaceRight = replaceRight
}

func (sb *StatusBar) Snapshot() StatusSnapshot {
	return StatusSnapshot{
		Running:         sb.running,
		ModelName:       sb.modelName,
		Provider:        sb.provider,
		ReasoningEffort: sb.reasoningEffort,
		Mode:            sb.mode,
		Usage:           sb.usage,
		MaxInputTokens:  sb.maxInputTokens,
	}
}

func (sb *StatusBar) HandleKey(*tcell.EventKey) bool { return false }

func (sb *StatusBar) Draw(s tcell.Screen, bounds layout.Region, _ bool) {
	th := styles.Current()
	base := th.Base().Background(th.StatusBar)
	FillRegion(s, bounds, ' ', base)

	y := bounds.Top
	innerW := bounds.Width - 2
	if innerW < 1 {
		return
	}

	leftX := bounds.Left + 1

	if sb.replaceLeft {
		if len(sb.extraLeft) > 0 {
			DrawViewLine(s, leftX, y, bounds.Right()-leftX-1, pkgplugin.ViewLine{Spans: sb.extraLeft}, base)
		}
	} else {
		if sb.mode != "" {
			label := i18n.T("mode." + sb.mode)
			DrawText(s, leftX, y, label, base.Foreground(modeColor(sb.mode)).Bold(true))
			leftX += len(label) + 1
		}
		if sb.running {
			DrawText(s, leftX, y, "●", base.Foreground(th.Accent).Bold(true))
		} else {
			DrawText(s, leftX, y, "●", base.Foreground(th.Hint))
		}
		leftX += 2
		if sb.modelName != "" {
			DrawText(s, leftX, y, sb.modelName, base.Foreground(th.Foreground))
			leftX += len(sb.modelName)
		}
		if sb.provider != "" {
			DrawText(s, leftX, y, "/"+sb.provider, base.Foreground(th.Hint))
			leftX += len(sb.provider) + 1
		}
		if sb.reasoningEffort != "" {
			DrawText(s, leftX, y, " ["+sb.reasoningEffort+"]", base.Foreground(th.Accent))
			leftX += len(sb.reasoningEffort) + 3
		}
		if len(sb.extraLeft) > 0 && leftX < bounds.Right()-1 {
			DrawText(s, leftX, y, " · ", base.Foreground(th.Hint))
			leftX += 3
			DrawViewLine(s, leftX, y, bounds.Right()-leftX-1, pkgplugin.ViewLine{Spans: sb.extraLeft}, base)
		}
	}

	rightX := bounds.Right() - 1

	type segment struct {
		text  string
		style tcell.Style
	}
	var segs []segment

	if !sb.replaceRight {
		segs = append(segs, segment{"Ctrl+P", base.Foreground(th.Foreground)})

		if sb.usage.Cost > 0 {
			segs = append(segs, segment{fmt.Sprintf("$%.4f", sb.usage.Cost), base.Foreground(th.Foreground)})
		}

		if sb.usage.TotalTokens > 0 {
			segs = append(segs, segment{llm.FormatContextUsage(sb.usage, sb.maxInputTokens), base.Foreground(th.Foreground)})
		}
	}
	for _, span := range sb.extraRight {
		segs = append(segs, segment{span.Text, ViewStyle(span.Style, base)})
	}

	for i, seg := range segs {
		x := rightX - len(seg.text)
		if x >= bounds.Left {
			DrawText(s, x, y, seg.text, seg.style)
		}
		rightX -= len(seg.text)
		if i < len(segs)-1 {
			rightX -= 1
			if rightX >= bounds.Left {
				s.SetContent(rightX, y, '·', nil, base.Foreground(th.Hint))
			}
			rightX -= 1
		}
	}
}

func modeColor(mode string) tcell.Color {
	switch mode {
	case "plan":
		return tcell.ColorGold
	case "build":
		return tcell.ColorDodgerBlue
	case "auto":
		return tcell.ColorMediumSpringGreen
	default:
		return tcell.ColorGray
	}
}
