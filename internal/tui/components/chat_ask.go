package components

import (
	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

type askQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Required bool     `json:"required"`
}

type askArgs struct {
	Questions []askQuestion `json:"questions"`
}

type askOutput struct {
	Answers map[string]string `json:"answers"`
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

func (c *Chat) askCardLines(it *ChatItem, width int) []Line {
	th := styles.Current()
	if width < 12 {
		return nil
	}

	border := th.Base().Foreground(th.Border).Background(th.Background)
	accent := th.Base().Foreground(th.Accent).Background(th.Background)
	dim := th.Base().Foreground(th.TextDim).Background(th.Background)

	var qs []askQuestion
	var args askArgs
	if err := json.Unmarshal([]byte(it.ToolArgs), &args); err == nil {
		qs = args.Questions
	}

	var answers map[string]string
	if it.ToolOutput != "" {
		var out askOutput
		if err := json.Unmarshal([]byte(it.ToolOutput), &out); err == nil {
			answers = out.Answers
		}
	}
	if answers == nil {
		answers = make(map[string]string)
	}

	var lines []Line

	top := Line{{R: '╭', S: border}}
	top = append(top, Cell{R: '─', S: border})
	for _, r := range i18n.T("chat.card.ask") {
		top = append(top, Cell{R: r, S: accent})
	}
	for top.Width() < width-1 {
		top = append(top, Cell{R: '─', S: border})
	}
	top = append(top, Cell{R: '╮', S: border})
	lines = append(lines, top)

	for _, q := range qs {
		qtext := fitCardText(q.Question, width-6)
		qLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
		for _, r := range "Q: " + qtext {
			qLine = append(qLine, Cell{R: r, S: accent})
		}
		for qLine.Width() < width-1 {
			qLine = append(qLine, Cell{R: ' ', S: th.Base()})
		}
		qLine = append(qLine, Cell{R: '│', S: border})
		lines = append(lines, qLine)

		a := answers[q.ID]
		if a == "" {
			a = i18n.T("chat.ask.unanswered")
		}
		atext := fitCardText(a, width-7)
		aLine := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}, {R: ' ', S: th.Base()}}
		for _, r := range "A: " + atext {
			aLine = append(aLine, Cell{R: r, S: dim})
		}
		for aLine.Width() < width-1 {
			aLine = append(aLine, Cell{R: ' ', S: th.Base()})
		}
		aLine = append(aLine, Cell{R: '│', S: border})
		lines = append(lines, aLine)
	}

	lines = append(lines, cardSep(border, width))

	nAnswered := 0
	for _, q := range qs {
		if answers[q.ID] != "" {
			nAnswered++
		}
	}
	summary := fitCardText(i18n.Tf("chat.ask.answered", nAnswered, len(qs)), width-3)
	lines = append(lines, cardFooter(border, dim, th.Base(), width, summary))
	lines = append(lines, bottomEdge(border, width))

	return lines
}
