package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/layout"
)

func askKey(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, 0) }

func askRune(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func askScreenText(s tcell.Screen, w, h int) string {
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			main, _, _, _ := s.GetContent(x, y)
			b.WriteRune(main)
		}
		b.WriteRune('\n')
	}
	return b.String()
}

func TestAskPickerReasonFlow(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) {
		got = answers
	})
	p.SetQuestions([]AskQuestion{
		{ID: "decision", Question: "Allow?", Type: "select", Options: []string{"Allow", "Allow All", "Reject"}, Required: true},
	})

	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("a single question should submit without a confirm page")
	}
	if got == nil || got["decision"] != "Allow" {
		t.Fatalf("answers = %v, want decision=Allow", got)
	}
}

func TestAskPickerSingleQuestionSkipsConfirm(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "decision", Question: "Allow?", Type: "select", Options: []string{"Allow", "Reject"}, Required: true},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.confirming {
		t.Fatal("single question must not enter the confirm page")
	}
	if p.Active() {
		t.Fatal("single question should submit on the first Enter")
	}
}

func TestAskPickerMultiQuestionConfirms(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) { got = answers })
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}, Required: true},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}, Required: true},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.Active() || p.confirming {
		t.Fatal("Enter should select the first answer and advance, not confirm yet")
	}
	if p.tab != 1 {
		t.Fatalf("tab = %d, want 1 after Enter", p.tab)
	}
	if got != nil {
		t.Fatalf("must not submit before confirming, got %v", got)
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.Active() || !p.confirming {
		t.Fatal("the last question should open the confirm page")
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
	if got["a"] != "A1" || got["b"] != "B1" {
		t.Fatalf("answers = %v, want a=A1 b=B1", got)
	}
}

func TestAskPickerSingleTextReason(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) {
		got = answers
	})
	p.SetQuestions([]AskQuestion{
		{ID: "reason", Question: "Reason?", Type: "text"},
	})

	st := p.states[p.tab]
	if st.input == nil || !st.input.focused {
		t.Fatal("text input should exist and be focused")
	}

	for _, r := range "not today" {
		if !p.HandleKey(askRune(r)) {
			t.Fatalf("rune %q not handled", r)
		}
	}
	if v := st.input.Value(); v != "not today" {
		t.Fatalf("value = %q, want %q", v, "not today")
	}

	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
	if got == nil || got["reason"] != "not today" {
		t.Fatalf("answers = %v, want reason=not today", got)
	}
}

func TestAskPickerTextCaretMoves(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "text"},
		{ID: "b", Question: "Second?", Type: "text"},
	})
	for _, r := range "abc" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyLeft))
	p.HandleKey(askKey(tcell.KeyLeft))
	p.HandleKey(askRune('X'))

	if got := p.states[0].input.Value(); got != "aXbc" {
		t.Fatalf("value = %q, want %q (← must move the caret, not switch questions)", got, "aXbc")
	}
	if p.tab != 0 {
		t.Fatalf("tab = %d, want 0 (← must not switch questions in a text field)", p.tab)
	}
}

func TestAskPickerTabSwitchesQuestions(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "text"},
		{ID: "b", Question: "Second?", Type: "text"},
	})
	p.HandleKey(askKey(tcell.KeyTab))
	if p.tab != 1 {
		t.Fatalf("tab = %d, want 1 after Tab", p.tab)
	}
	p.HandleKey(askKey(tcell.KeyBacktab))
	if p.tab != 0 {
		t.Fatalf("tab = %d, want 0 after Shift+Tab", p.tab)
	}
}

func TestAskPickerInsertPaste(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{{ID: "a", Question: "Body?", Type: "text"}})
	p.InsertPaste("hello\nworld")
	if got := p.states[0].input.Value(); got != "hello\nworld" {
		t.Fatalf("value = %q, want %q", got, "hello\nworld")
	}
}

func TestAskPickerTypedTextVisible(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "reason", Question: "Reason?", Type: "text"},
	})
	for _, r := range "not today" {
		p.HandleKey(askRune(r))
	}

	bounds := layout.Region{Left: 10, Top: 2, Width: 80, Height: p.Height()}
	p.Draw(s, bounds)

	if all := askScreenText(s, 100, 30); !strings.Contains(all, "not today") {
		t.Fatalf("typed text not visible:\n%s", all)
	}
}

func TestAskPickerLongOptionWraps(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(60, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "q", Question: "Pick", Type: "select", Options: []string{"alpha beta gamma delta epsilon zeta eta theta iota kappa lambda"}},
	})
	bounds := layout.Region{Left: 2, Top: 2, Width: 40, Height: p.Height()}
	p.Draw(s, bounds)

	if all := askScreenText(s, 60, 30); !strings.Contains(all, "lambda") {
		t.Fatalf("long option was clipped; tail not rendered:\n%s", all)
	}
}

func TestAskPickerUnknownTypeFallsBackToText(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	sent := false
	p.SetOnSend(func(answers map[string]string) {
		got = answers
		sent = true
	})
	p.SetQuestions([]AskQuestion{
		{ID: "q1", Question: "Which?", Type: "checkbox"},
	})
	if !p.Active() {
		t.Fatal("picker should activate for an unknown type")
	}
	if p.states[0].input == nil {
		t.Fatal("unknown type should fall back to a text field")
	}

	for _, r := range "yes" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
	if !sent || got["q1"] != "yes" {
		t.Fatalf("answers = %v, want q1=yes", got)
	}
}

func TestAskPickerEmptyQuestionsDeactivates(t *testing.T) {
	p := NewAskPicker()
	done := false
	p.SetOnDone(func() { done = true })
	p.SetQuestions(nil)
	if p.Active() {
		t.Fatal("picker must not activate without questions")
	}
	if !done {
		t.Fatal("onDone should be called for empty questions")
	}
}

func TestAskPickerEscCancels(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	sent := false
	p.SetOnSend(func(answers map[string]string) {
		got = answers
		sent = true
	})
	p.SetQuestions([]AskQuestion{
		{ID: "q1", Question: "Which?", Type: "checkbox"},
	})
	if !p.HandleKey(askKey(tcell.KeyEsc)) {
		t.Fatal("Esc should be handled")
	}
	if p.Active() {
		t.Fatal("Esc should cancel the picker")
	}
	if !sent || got != nil {
		t.Fatalf("cancel should send nil answers, got sent=%v answers=%v", sent, got)
	}
}

func TestAskPickerMultiSelect(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	sent := false
	p.SetOnSend(func(answers map[string]string) {
		got = answers
		sent = true
	})
	p.SetQuestions([]AskQuestion{
		{ID: "tags", Question: "Tags?", Type: "multiselect", Options: []string{"a", "b", "c"}},
	})

	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askKey(tcell.KeyTab))

	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
	if !sent || got["tags"] != "a, b" {
		t.Fatalf("answers = %v, want tags=\"a, b\"", got)
	}
}

func TestAskPickerMultiSelectEnterTogglesAndStays(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "tags", Question: "Tags?", Type: "multiselect", Options: []string{"a", "b"}},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.tab != 0 || !p.Active() {
		t.Fatal("Enter on a multiselect must toggle and stay on the question")
	}
	if !p.states[0].multi["a"] {
		t.Fatal("Enter should toggle the highlighted option")
	}
}

func TestAskPickerMultiSelectRequiredNeedsSelection(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "tags", Question: "Tags?", Type: "multiselect", Options: []string{"a", "b"}, Required: true},
	})
	if !p.HandleKey(askKey(tcell.KeyTab)) {
		t.Fatal("Tab should be handled")
	}
	if !p.Active() || p.confirming {
		t.Fatal("required multiselect must block advancing without a selection")
	}
	if p.states[0].err == "" {
		t.Fatal("required multiselect should show an error")
	}
}

func TestAskPickerBoolean(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) {
		got = answers
	})
	p.SetQuestions([]AskQuestion{
		{ID: "proceed", Question: "Proceed?", Type: "boolean"},
	})

	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
	if got["proceed"] != "yes" {
		t.Fatalf("answers = %v, want proceed=yes", got)
	}
}

func TestAskPickerConfirmEscGoesBack(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "text"},
		{ID: "b", Question: "Second?", Type: "text"},
	})
	p.HandleKey(askRune('x'))

	p.HandleKey(askKey(tcell.KeyEnter))
	if p.confirming || p.tab != 1 {
		t.Fatal("Enter should advance to the next question")
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.confirming {
		t.Fatal("the last question should open the confirm page")
	}
	p.HandleKey(askKey(tcell.KeyEsc))
	if !p.Active() || p.confirming {
		t.Fatal("Esc on the confirm page should return to the questions")
	}
	if got := p.states[0].input.Value(); got != "x" {
		t.Fatalf("value = %q, want x (answer must be preserved)", got)
	}

	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.confirming {
		t.Fatal("Enter on the last question should reopen the confirm page")
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("picker should close after confirming")
	}
}

func TestAskPickerConfirmShowsQuestionsAndAnswers(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.confirming {
		t.Fatal("the last question should open the confirm page")
	}

	bounds := layout.Region{Left: 4, Top: 2, Width: 60, Height: p.Height()}
	p.Draw(s, bounds)

	all := askScreenText(s, 100, 30)
	for _, want := range []string{"Review your answers", "1. First?", "A1", "2. Second?", "B1"} {
		if !strings.Contains(all, want) {
			t.Fatalf("confirm page missing %q:\n%s", want, all)
		}
	}
}

func TestAskPickerSelectUsesEnter(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red", "Green", "Blue"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.states[0].answer != "Green" {
		t.Fatalf("answer = %q, want Green", p.states[0].answer)
	}
	if p.Active() {
		t.Fatal("a single question should submit after Enter")
	}
}

func TestAskPickerSelectEnterAdvances(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.tab != 1 || p.confirming {
		t.Fatal("Enter should select and advance to the next question")
	}
	if got := p.states[0].answer; got != "A1" {
		t.Fatalf("answer = %q, want A1", got)
	}
}

func TestAskPickerSpaceDoesNothing(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}},
	})
	if !p.HandleKey(askRune(' ')) {
		t.Fatal("Space should be consumed")
	}
	if p.tab != 0 || p.confirming {
		t.Fatal("Space must not advance to the next question")
	}
	if p.states[0].answer != "" {
		t.Fatalf("Space must not select an answer, got %q", p.states[0].answer)
	}
}

func TestAskPickerTabOnLastFinishes(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}},
	})
	p.HandleKey(askKey(tcell.KeyTab))
	if p.tab != 1 || p.confirming {
		t.Fatal("Tab should move to the next question")
	}
	p.HandleKey(askKey(tcell.KeyTab))
	if !p.Active() || !p.confirming {
		t.Fatal("Tab on the last question should open the confirm page")
	}
}

func TestAskPickerCustomOptionSelect(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) { got = answers })
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red", "Blue"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyDown))
	if !p.states[0].onCustom() {
		t.Fatal("cursor should be on the Custom row")
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.states[0].customEditing {
		t.Fatal("Enter on the Custom row should open the editor")
	}
	for _, r := range "Purple" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("a single question should submit after applying the custom answer")
	}
	if got["c"] != "Purple" {
		t.Fatalf("answers = %v, want c=Purple", got)
	}
}

func TestAskPickerCustomOptionMultiselect(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) { got = answers })
	p.SetQuestions([]AskQuestion{
		{ID: "tags", Question: "Tags?", Type: "multiselect", Options: []string{"a", "b"}},
	})
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	for _, r := range "z" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.Active() || p.states[0].customEditing {
		t.Fatal("applying a custom multiselect answer should return to the options")
	}
	if p.states[0].customValue != "z" {
		t.Fatalf("customValue = %q, want z", p.states[0].customValue)
	}
	p.HandleKey(askKey(tcell.KeyTab))
	if p.Active() {
		t.Fatal("Tab should advance and submit the single question")
	}
	if got["tags"] != "a, z" {
		t.Fatalf("answers = %v, want tags=\"a, z\"", got)
	}
}

func TestAskPickerCustomOptionBoolean(t *testing.T) {
	p := NewAskPicker()
	var got map[string]string
	p.SetOnSend(func(answers map[string]string) { got = answers })
	p.SetQuestions([]AskQuestion{
		{ID: "proceed", Question: "Proceed?", Type: "boolean"},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	for _, r := range "maybe" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.Active() {
		t.Fatal("a single question should submit after applying the custom answer")
	}
	if got["proceed"] != "maybe" {
		t.Fatalf("answers = %v, want proceed=maybe", got)
	}
}

func TestAskPickerCustomEscCancels(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red", "Blue"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askRune('x'))
	p.HandleKey(askKey(tcell.KeyEsc))
	if !p.Active() {
		t.Fatal("Esc in the custom editor should not close the picker")
	}
	if p.states[0].customEditing {
		t.Fatal("Esc should close the custom editor")
	}
	if p.states[0].answer != "" {
		t.Fatalf("Esc must not apply the custom answer, got %q", p.states[0].answer)
	}
}

func TestAskPickerCustomEmptyRejected(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.states[0].customEditing {
		t.Fatal("an empty custom answer must keep the editor open")
	}
	if p.states[0].err == "" {
		t.Fatal("an empty custom answer should show an error")
	}
}

func TestAskPickerCustomSpaceTypes(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	p.HandleKey(askRune('a'))
	p.HandleKey(askRune(' '))
	p.HandleKey(askRune('b'))
	if got := p.states[0].custom.Value(); got != "a b" {
		t.Fatalf("custom value = %q, want %q", got, "a b")
	}
	if p.tab != 0 || p.confirming {
		t.Fatal("Space in the custom editor must not advance")
	}
}

func TestAskPickerCustomRowRendered(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red", "Blue"}},
	})
	bounds := layout.Region{Left: 4, Top: 2, Width: 50, Height: p.Height()}
	p.Draw(s, bounds)

	if all := askScreenText(s, 80, 30); !strings.Contains(all, "Custom") {
		t.Fatalf("Custom row not rendered:\n%s", all)
	}
}

func TestAskPickerCustomEditorVisible(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	for _, r := range "zzz" {
		p.HandleKey(askRune(r))
	}
	bounds := layout.Region{Left: 4, Top: 2, Width: 50, Height: p.Height()}
	p.Draw(s, bounds)

	if all := askScreenText(s, 80, 30); !strings.Contains(all, "zzz") {
		t.Fatalf("custom editor text not rendered:\n%s", all)
	}
}

func TestAskPickerCustomPaste(t *testing.T) {
	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "c", Question: "Color?", Type: "select", Options: []string{"Red"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	p.InsertPaste("hello")
	if got := p.states[0].custom.Value(); got != "hello" {
		t.Fatalf("custom value = %q, want hello", got)
	}
}

func TestAskPickerCustomConfirmShowsValue(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 30)

	p := NewAskPicker()
	p.SetQuestions([]AskQuestion{
		{ID: "a", Question: "First?", Type: "select", Options: []string{"A1", "A2"}},
		{ID: "b", Question: "Second?", Type: "select", Options: []string{"B1", "B2"}},
	})
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyDown))
	p.HandleKey(askKey(tcell.KeyEnter))
	for _, r := range "Zed" {
		p.HandleKey(askRune(r))
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if p.tab != 1 {
		t.Fatal("applying a custom select answer should advance")
	}
	p.HandleKey(askKey(tcell.KeyEnter))
	if !p.confirming {
		t.Fatal("the last question should open the confirm page")
	}

	bounds := layout.Region{Left: 4, Top: 2, Width: 60, Height: p.Height()}
	p.Draw(s, bounds)

	if all := askScreenText(s, 100, 30); !strings.Contains(all, "Zed") {
		t.Fatalf("confirm page missing the custom answer:\n%s", all)
	}
}
