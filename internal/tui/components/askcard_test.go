package components

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/styles"
)

func askCardItem() *ChatItem {
	return &ChatItem{
		Kind:       ItemTool,
		ToolName:   "askuserquestion",
		ToolArgs:   `{"questions":[{"id":"scope","question":"Scope?","type":"select","options":["a","b"],"required":true},{"id":"time","question":"Timeline?","type":"text","required":false}]}`,
		ToolOutput: `{"answers":{"scope":"yes"}}`,
	}
}

func askCardItemFor(question, answer string) *ChatItem {
	args, err := json.Marshal(map[string]any{
		"questions": []map[string]any{{
			"id":       "q",
			"question": question,
			"type":     "text",
			"required": true,
		}},
	})
	if err != nil {
		panic(err)
	}
	out, err := json.Marshal(map[string]any{"answers": map[string]string{"q": answer}})
	if err != nil {
		panic(err)
	}
	return &ChatItem{
		Kind:       ItemTool,
		ToolName:   "askuserquestion",
		ToolArgs:   string(args),
		ToolOutput: string(out),
	}
}

func setupAskCard(t *testing.T) *Chat {
	t.Helper()
	styles.RegisterDefaults()
	styles.Set("dark")
	prev := i18n.Current()
	if !i18n.Set("en") {
		t.Fatalf("failed to switch i18n to en")
	}
	t.Cleanup(func() { i18n.Set(prev) })
	return NewChat()
}

func TestAskCardSnapshot(t *testing.T) {
	c := setupAskCard(t)
	got := flattenLines(c.askCardLines(askCardItem(), 40))
	want := strings.Join([]string{
		"╭─ ASK ────────────────────────────────╮",
		"│ Q: Scope?                            │",
		"│  A: yes                              │",
		"│ Q: Timeline?                         │",
		"│  A: (unanswered)                     │",
		"├──────────────────────────────────────┤",
		"│ 1 / 2 answered                       │",
		"╰──────────────────────────────────────╯",
	}, "\n")
	if got != want {
		t.Fatalf("unexpected ask card layout:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAskCardFitsEveryWidth(t *testing.T) {
	c := setupAskCard(t)
	it := askCardItem()

	for w := 12; w <= 100; w++ {
		lines := c.askCardLines(it, w)
		if len(lines) == 0 {
			t.Fatalf("width %d: expected lines, got none", w)
		}
		for i, l := range lines {
			if lw := l.Width(); lw > w {
				t.Fatalf("width %d: line %d is %d cells wide: %q", w, i, lw, flattenLines([]Line{l}))
			}
		}
	}

	if lines := c.askCardLines(it, 11); lines != nil {
		t.Fatalf("narrow widths should render nothing, got:\n%s", flattenLines(lines))
	}
}

func TestAskCardKeepsMultibyteIntact(t *testing.T) {
	c := setupAskCard(t)
	long := strings.Repeat("ğ", 40)
	out := flattenLines(c.askCardLines(askCardItemFor(long, long), 40))

	if strings.ContainsRune(out, '\uFFFD') {
		t.Fatalf("truncation split a multibyte rune:\n%s", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("long text should be shortened with an ellipsis:\n%s", out)
	}
	if !strings.Contains(out, strings.Repeat("ğ", 10)) {
		t.Fatalf("truncation lost visible text:\n%s", out)
	}
	for i, l := range c.askCardLines(askCardItemFor(long, long), 40) {
		if lw := l.Width(); lw > 40 {
			t.Fatalf("line %d is %d cells wide: %q", i, lw, flattenLines([]Line{l}))
		}
	}
}
