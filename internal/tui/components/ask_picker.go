package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

type AskQuestion struct {
	ID       string
	Question string
	Type     string
	Options  []string
	Required bool
}

type AskPicker struct {
	active     bool
	confirming bool
	questions  []AskQuestion
	states     []*askState
	tab        int

	onSend func(answers map[string]string)
	onDone func()
}

type askState struct {
	q      AskQuestion
	input  *Input
	values []string
	cursor int
	scroll int
	answer string
	multi  map[string]bool
	err    string

	custom        *Input
	customEditing bool
	customValue   string
}

func (st *askState) hasCustom() bool {
	switch st.q.Type {
	case "select", "multiselect", "boolean":
		return true
	}
	return false
}

func (st *askState) optionCount() int {
	n := len(st.q.Options)
	if st.hasCustom() {
		n++
	}
	return n
}

func (st *askState) onCustom() bool {
	return st.hasCustom() && st.cursor >= len(st.q.Options)
}

func NewAskPicker() *AskPicker { return &AskPicker{} }

func (a *AskPicker) SetOnSend(fn func(map[string]string)) { a.onSend = fn }
func (a *AskPicker) SetOnDone(fn func())                  { a.onDone = fn }
func (a *AskPicker) Active() bool                         { return a.active }

func (a *AskPicker) Clear() {
	onSend, onDone := a.onSend, a.onDone
	*a = AskPicker{onSend: onSend, onDone: onDone}
}

func (a *AskPicker) SetQuestions(qs []AskQuestion) {
	onSend, onDone := a.onSend, a.onDone
	if len(qs) == 0 {
		a.Clear()
		if onDone != nil {
			onDone()
		}
		return
	}
	*a = AskPicker{active: true, questions: qs, onSend: onSend, onDone: onDone}
	for i := range qs {
		st := &askState{q: qs[i]}
		switch st.q.Type {
		case "select":
			if len(st.q.Options) > 0 {
				st.values = append([]string(nil), st.q.Options...)
			} else {
				a.textState(st)
			}
		case "multiselect":
			if len(st.q.Options) > 0 {
				st.values = append([]string(nil), st.q.Options...)
				st.multi = make(map[string]bool)
			} else {
				a.textState(st)
			}
		case "boolean":
			st.q.Options = []string{i18n.T("ask.yes"), i18n.T("ask.no")}
			st.values = []string{"yes", "no"}
		default:
			a.textState(st)
		}
		a.states = append(a.states, st)
	}
	a.focusCurrent()
}

func (a *AskPicker) textState(st *askState) {
	st.q.Type = "text"
	in := NewInput()
	in.SetRows(1, 2)
	in.Placeholder = i18n.T("ask.placeholder")
	st.input = in
}

func (a *AskPicker) current() *askState {
	if a.tab < 0 || a.tab >= len(a.states) {
		return nil
	}
	return a.states[a.tab]
}

func (a *AskPicker) next() {
	if len(a.states) == 0 {
		return
	}
	a.tab = (a.tab + 1) % len(a.states)
	a.focusCurrent()
}

func (a *AskPicker) prev() {
	if len(a.states) == 0 {
		return
	}
	a.tab = (a.tab - 1 + len(a.states)) % len(a.states)
	a.focusCurrent()
}

func (a *AskPicker) tabNext() {
	st := a.current()
	if st != nil && st.q.Type != "text" && a.tab == len(a.states)-1 {
		a.advance(st)
		return
	}
	a.next()
}

func (a *AskPicker) focusCurrent() {
	for i, st := range a.states {
		if st.input != nil {
			if i == a.tab {
				st.input.Focus()
			} else {
				st.input.Blur()
			}
		}
		if st.custom != nil {
			if i == a.tab && st.customEditing {
				st.custom.Focus()
			} else {
				st.custom.Blur()
			}
		}
	}
}

func (st *askState) answered() bool {
	switch st.q.Type {
	case "text":
		return st.input != nil && strings.TrimSpace(st.input.Value()) != ""
	case "multiselect":
		if strings.TrimSpace(st.customValue) != "" {
			return true
		}
		for _, v := range st.multi {
			if v {
				return true
			}
		}
		return false
	default:
		return st.answer != ""
	}
}

func (a *AskPicker) answerOf(st *askState) string {
	switch st.q.Type {
	case "text":
		if st.input == nil {
			return ""
		}
		return strings.TrimSpace(st.input.Value())
	case "multiselect":
		sel := make([]string, 0, len(st.q.Options)+1)
		for _, opt := range st.q.Options {
			if st.multi[opt] {
				sel = append(sel, opt)
			}
		}
		if v := strings.TrimSpace(st.customValue); v != "" {
			sel = append(sel, v)
		}
		return strings.Join(sel, ", ")
	default:
		return st.answer
	}
}

func (a *AskPicker) moveCursor(st *askState, delta int) {
	n := st.optionCount()
	if n == 0 {
		return
	}
	st.cursor = (st.cursor + delta + n) % n
	st.err = ""
}

func (a *AskPicker) selectCurrent(st *askState) {
	if st.cursor < 0 || st.cursor >= len(st.values) {
		return
	}
	st.answer = st.values[st.cursor]
	st.err = ""
}

func (a *AskPicker) toggleCurrent(st *askState) {
	if st.cursor < 0 || st.cursor >= len(st.q.Options) {
		return
	}
	label := st.q.Options[st.cursor]
	st.multi[label] = !st.multi[label]
	st.err = ""
}

func (a *AskPicker) collect() map[string]string {
	answers := make(map[string]string)
	for _, st := range a.states {
		if v := a.answerOf(st); v != "" {
			answers[st.q.ID] = v
		}
	}
	return answers
}

func (a *AskPicker) firstMissingRequired() int {
	for i, st := range a.states {
		if st.q.Required && !st.answered() {
			return i
		}
	}
	return -1
}

func (a *AskPicker) advance(st *askState) {
	if st.q.Required && !st.answered() {
		st.err = i18n.T("ask.required")
		return
	}
	if a.tab < len(a.states)-1 {
		a.next()
		return
	}
	if i := a.firstMissingRequired(); i >= 0 {
		a.states[i].err = i18n.T("ask.required")
		a.tab = i
		a.confirming = false
		a.focusCurrent()
		return
	}
	if len(a.states) == 1 {
		a.finalSubmit()
		return
	}
	a.confirming = true
}

func (a *AskPicker) openCustom(st *askState) {
	if st.custom == nil {
		in := NewInput()
		in.SetRows(1, 1)
		in.Placeholder = i18n.T("ask.custom.placeholder")
		st.custom = in
	}
	st.customEditing = true
	st.err = ""
	st.custom.Focus()
}

func (a *AskPicker) closeCustom(st *askState) {
	if st.custom != nil {
		st.custom.Blur()
	}
	st.customEditing = false
}

func (a *AskPicker) commitCustom(st *askState) {
	if st.custom == nil {
		return
	}
	v := strings.TrimSpace(st.custom.Value())
	if v == "" {
		st.err = i18n.T("ask.custom.empty")
		return
	}
	st.customValue = v
	st.err = ""
	a.closeCustom(st)
	if st.q.Type == "multiselect" {
		return
	}
	st.answer = v
	a.advance(st)
}

func (a *AskPicker) finalSubmit() {
	answers := a.collect()
	if a.onSend != nil {
		a.onSend(answers)
	}
	a.active = false
	if a.onDone != nil {
		a.onDone()
	}
}

func (a *AskPicker) cancel() {
	if a.onSend != nil {
		a.onSend(nil)
	}
	a.active = false
	if a.onDone != nil {
		a.onDone()
	}
}

func (a *AskPicker) InsertPaste(text string) {
	st := a.current()
	if st == nil {
		return
	}
	var in *Input
	switch {
	case st.customEditing:
		in = st.custom
	case st.q.Type == "text":
		in = st.input
	}
	if in == nil {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			in.Newline()
		}
		for _, r := range line {
			in.InsertRune(r)
		}
	}
}

func (a *AskPicker) HandleKey(ev *tcell.EventKey) bool {
	if !a.active {
		return false
	}

	if a.confirming {
		switch ev.Key() {
		case tcell.KeyEsc:
			a.confirming = false
			a.focusCurrent()
		case tcell.KeyEnter:
			a.finalSubmit()
		}
		return true
	}

	st := a.current()
	if st == nil {
		return false
	}

	if st.customEditing {
		switch ev.Key() {
		case tcell.KeyEsc:
			a.closeCustom(st)
			return true
		case tcell.KeyEnter:
			a.commitCustom(st)
			return true
		case tcell.KeyBacktab:
			a.closeCustom(st)
			a.prev()
			return true
		case tcell.KeyTab:
			a.closeCustom(st)
			if ev.Modifiers()&tcell.ModShift != 0 {
				a.prev()
			} else {
				a.tabNext()
			}
			return true
		}
		if st.custom != nil && st.custom.HandleKey(ev) {
			return true
		}
		return false
	}

	switch ev.Key() {
	case tcell.KeyEsc:
		a.cancel()
		return true
	case tcell.KeyBacktab:
		a.prev()
		return true
	case tcell.KeyTab:
		if ev.Modifiers()&tcell.ModShift != 0 {
			a.prev()
		} else {
			a.tabNext()
		}
		return true
	}

	if st.q.Type == "text" {
		if st.input == nil {
			return false
		}
		if ev.Key() == tcell.KeyEnter && ev.Modifiers()&tcell.ModShift == 0 {
			a.advance(st)
			return true
		}
		return st.input.HandleKey(ev)
	}

	switch ev.Key() {
	case tcell.KeyUp:
		a.moveCursor(st, -1)
		return true
	case tcell.KeyDown:
		a.moveCursor(st, 1)
		return true
	case tcell.KeyLeft:
		a.prev()
		return true
	case tcell.KeyRight:
		a.next()
		return true
	case tcell.KeyEnter:
		if st.onCustom() {
			a.openCustom(st)
			return true
		}
		if st.q.Type == "multiselect" {
			a.toggleCurrent(st)
			return true
		}
		a.selectCurrent(st)
		a.advance(st)
		return true
	case tcell.KeyRune:
		if ev.Rune() == ' ' {
			return true
		}
	}
	return false
}

func (a *AskPicker) Height() int {
	if !a.active {
		return 0
	}
	if a.confirming {
		n := len(a.states)
		if n < 1 {
			n = 1
		}
		return 2*n + 4
	}
	st := a.current()
	if st == nil {
		return 0
	}
	body := 4
	if st.q.Type != "text" {
		body = st.optionCount()
		if body < 3 {
			body = 3
		}
		if body > 8 {
			body = 8
		}
		if st.customEditing {
			body += 3
		}
	}
	return 8 + body
}

func askTabLabel(q AskQuestion, idx int) string {
	words := strings.Fields(q.Question)
	if len(words) == 0 {
		return fmt.Sprintf("Q%d", idx+1)
	}
	label := strings.Join(words, " ")
	if len(words) > 3 {
		label = strings.Join(words[:3], " ") + "…"
	}
	label = TruncateTo(label, 20)
	return label
}

func askRestyle(l Line, st tcell.Style) Line {
	out := make(Line, len(l))
	for i, c := range l {
		out[i] = Cell{R: c.R, S: st}
	}
	return out
}

func (a *AskPicker) Draw(s tcell.Screen, bounds layout.Region) {
	if !a.active || bounds.Width < 8 || bounds.Height < 4 {
		return
	}
	th := styles.Current()
	FillRegion(s, bounds, ' ', th.Base().Background(th.InputBg))
	DrawBox(s, bounds, th.Base().Foreground(th.Border))

	title := i18n.Tf("ask.title", a.tab+1, len(a.states))
	if a.confirming {
		title = i18n.T("ask.confirm")
	}
	DrawText(s, bounds.Left+2, bounds.Top, title, th.Base().Foreground(th.Accent).Background(th.InputBg))

	inner := layout.Region{
		Left:   bounds.Left + 2,
		Top:    bounds.Top + 1,
		Width:  bounds.Width - 4,
		Height: bounds.Height - 2,
	}
	if inner.Width < 4 || inner.Height < 4 {
		return
	}

	if a.confirming {
		a.drawConfirm(s, inner, th)
		return
	}

	st := a.current()
	if st == nil {
		return
	}

	footerY := inner.Bottom() - 1
	y := inner.Top

	cx := inner.Left
	for i, qs := range a.states {
		label := askTabLabel(qs.q, i)
		if qs.answered() {
			label = "✓ " + label
		}
		chip := " " + label + " "
		w := DisplayWidth(chip)
		if cx+w > inner.Right() {
			break
		}
		style := th.Base().Foreground(th.Muted).Background(th.InputBg)
		if i == a.tab {
			style = th.Base().Foreground(th.InputBg).Background(th.Accent)
		}
		DrawText(s, cx, y, chip, style)
		cx += w + 1
	}
	y += 2

	qLines := WrapText(st.q.Question, th.Base().Foreground(th.Foreground).Background(th.InputBg), inner.Width)
	if len(qLines) > 2 {
		qLines = qLines[:2]
	}
	for i, ln := range qLines {
		if y+i >= footerY {
			break
		}
		DrawLine(s, inner.Left, y+i, ln)
	}
	y += len(qLines) + 1

	bodyTop := y
	bodyH := footerY - bodyTop
	if bodyH < 1 {
		bodyTop = footerY - 1
		bodyH = 1
	}
	body := layout.Region{Left: inner.Left, Top: bodyTop, Width: inner.Width, Height: bodyH}

	if st.q.Type == "text" {
		st.input.SetInnerWidth(body.Width - 2)
		st.input.Draw(s, body, true)
	} else if st.customEditing && st.custom != nil && body.Height >= 5 {
		editorH := 3
		optsH := body.Height - editorH
		a.drawOptions(s, st, layout.Region{Left: body.Left, Top: body.Top, Width: body.Width, Height: optsH})
		editor := layout.Region{Left: body.Left, Top: body.Top + optsH, Width: body.Width, Height: editorH}
		st.custom.SetInnerWidth(editor.Width - 2)
		st.custom.Draw(s, editor, true)
	} else {
		a.drawOptions(s, st, body)
	}

	multi := len(a.states) > 1
	nav := i18n.T("ask.footer.nav")
	if !multi {
		nav = ""
	}
	action := i18n.T("ask.footer.confirm")
	if !multi {
		action = i18n.T("ask.footer.submit")
	}
	cancel := i18n.T("ask.footer.cancel")
	hint := nav + i18n.T("ask.footer.select") + action + cancel
	switch {
	case st.customEditing:
		hint = nav + i18n.T("ask.custom.hint")
	case st.q.Type == "text":
		hint = nav + i18n.T("ask.footer.newline") + action + cancel
	case st.q.Type == "multiselect":
		hint = nav + i18n.T("ask.footer.move") + action + cancel
	}
	hintStyle := th.Base().Foreground(th.Hint).Background(th.InputBg)
	if st.err != "" {
		hint = st.err
		hintStyle = th.Base().Foreground(th.Error).Background(th.InputBg)
	}
	DrawText(s, inner.Left, footerY, TruncateTo(hint, inner.Width), hintStyle)
}

func (a *AskPicker) drawOptions(s tcell.Screen, st *askState, region layout.Region) {
	th := styles.Current()
	base := th.Base().Foreground(th.InputText).Background(th.InputBg)
	dim := th.Base().Foreground(th.Placeholder).Background(th.InputBg)

	total := st.optionCount()
	rows := make([]Line, 0, total)
	rowOf := make([]int, 0, total)
	cursorStart, cursorEnd := 0, 0
	for i := 0; i < total; i++ {
		custom := st.hasCustom() && i == len(st.q.Options)
		opt := ""
		if custom {
			opt = i18n.T("ask.custom")
			if st.customValue != "" {
				opt += ": " + st.customValue
			}
		} else {
			opt = st.q.Options[i]
		}

		on := false
		switch st.q.Type {
		case "multiselect":
			if custom {
				on = st.customValue != ""
			} else {
				on = st.multi[opt]
			}
		default:
			if custom {
				on = st.customValue != "" && st.answer == st.customValue
			} else if i < len(st.values) {
				on = st.answer != "" && st.values[i] == st.answer
			}
		}

		var marker string
		if st.q.Type == "multiselect" {
			if on {
				marker = "[x] "
			} else {
				marker = "[ ] "
			}
		} else {
			if on {
				marker = "(•) "
			} else {
				marker = "( ) "
			}
		}

		style := base
		if st.q.Type == "multiselect" && !on {
			style = dim
		}
		lines := WrapText(marker+opt, style, region.Width)
		if len(lines) == 0 {
			lines = []Line{nil}
		}
		if i == st.cursor {
			cursorStart = len(rows)
		}
		for _, ln := range lines {
			rows = append(rows, ln)
			rowOf = append(rowOf, i)
		}
		if i == st.cursor {
			cursorEnd = len(rows) - 1
		}
	}

	visible := region.Height
	if st.scroll > cursorStart {
		st.scroll = cursorStart
	}
	if cursorEnd >= st.scroll+visible {
		st.scroll = cursorEnd - visible + 1
	}
	if st.scroll < 0 {
		st.scroll = 0
	}
	if maxScroll := len(rows) - visible; st.scroll > maxScroll {
		st.scroll = maxScroll
		if st.scroll < 0 {
			st.scroll = 0
		}
	}

	cursorStyle := th.Base().Foreground(th.InputText).Background(th.Selection)
	for i := 0; i < visible; i++ {
		ri := st.scroll + i
		if ri >= len(rows) {
			break
		}
		ln := rows[ri]
		if rowOf[ri] == st.cursor {
			ln = askRestyle(ln, cursorStyle)
		}
		DrawLine(s, region.Left, region.Top+i, ln)
	}
}

func (a *AskPicker) drawConfirm(s tcell.Screen, inner layout.Region, th styles.Theme) {
	footerY := inner.Bottom() - 1
	qStyle := th.Base().Foreground(th.Muted).Background(th.InputBg)
	aStyle := th.Base().Foreground(th.InputText).Background(th.InputBg)
	headStyle := th.Base().Foreground(th.Accent).Background(th.InputBg)

	y := inner.Top
	if y < footerY {
		DrawText(s, inner.Left, y, TruncateTo(i18n.T("ask.review"), inner.Width), headStyle)
		y++
	}
	for i, st := range a.states {
		if y+1 >= footerY {
			break
		}
		label := fmt.Sprintf("%d. %s", i+1, st.q.Question)
		DrawText(s, inner.Left, y, TruncateTo(label, inner.Width), qStyle)
		ans := a.answerOf(st)
		if ans == "" {
			ans = "—"
		}
		DrawText(s, inner.Left+3, y+1, TruncateTo(ans, inner.Width-3), aStyle)
		y += 2
	}
	DrawText(s, inner.Left, footerY, TruncateTo(i18n.T("ask.confirm.footer"), inner.Width),
		th.Base().Foreground(th.Hint).Background(th.InputBg))
}
