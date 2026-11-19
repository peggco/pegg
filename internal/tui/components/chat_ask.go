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
		qtext := q.Question
		if len(qtext) > width-6 {
			qtext = qtext[:width-7] + "\u2026"
		}
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
		atext := a
		if len(atext) > width-6 {
			atext = atext[:width-7] + "\u2026"
		}
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

	sep := Line{{R: '├', S: border}}
	for i := 0; i < width-2; i++ {
		sep = append(sep, Cell{R: '─', S: border})
	}
	sep = append(sep, Cell{R: '┤', S: border})
	lines = append(lines, sep)

	nAnswered := 0
	for _, q := range qs {
		if answers[q.ID] != "" {
			nAnswered++
		}
	}
	footer := Line{{R: '│', S: border}, {R: ' ', S: th.Base()}}
	summary := i18n.Tf("chat.ask.answered", nAnswered, len(qs))
	for _, r := range summary {
		footer = append(footer, Cell{R: r, S: dim})
	}
	for footer.Width() < width-1 {
		footer = append(footer, Cell{R: ' ', S: th.Base()})
	}
	footer = append(footer, Cell{R: '│', S: border})
	lines = append(lines, footer)

	bot := Line{{R: '╰', S: border}}
	for i := 0; i < width-2; i++ {
		bot = append(bot, Cell{R: '─', S: border})
	}
	bot = append(bot, Cell{R: '╯', S: border})
	lines = append(lines, bot)

	return lines
}
