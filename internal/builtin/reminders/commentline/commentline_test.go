package commentline

import (
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestCountCommentLines(t *testing.T) {
	cases := []struct {
		name string
		path string
		text string
		want int
	}{
		{"go line", "a.go", "package a\n// hi\nfunc x() {}\n", 1},
		{"go block", "a.go", "/*\n * doc\n */\nfunc x() {}\n", 3},
		{"go inline not counted", "a.go", "x := 1 // set\n", 0},
		{"go directive not comment", "a.go", "#include <x>\n// c\n", 1},
		{"python", "a.py", "# hi\nx = 1\n", 1},
		{"ruby/shell", "a.sh", "# hi\nls\n", 1},
		{"sql", "a.sql", "-- hi\nSELECT 1;\n", 1},
		{"lua", "a.lua", "-- hi\nprint(1)\n", 1},
		{"html", "a.html", "<!-- hi -->\n<div></div>\n", 1},
		{"html multiline", "a.html", "<!--\n hi\n-->\n", 3},
		{"yaml", "a.yaml", "# c\nkey: v\n", 1},
		{"semantic", "a.lisp", ";; hi\n(foo)\n", 1},
		{"tex", "a.tex", "% hi\ntext\n", 1},
	}
	for _, c := range cases {
		if got := countCommentLines(c.text, styleForPath(c.path)); got != c.want {
			t.Errorf("%s: countCommentLines = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestAddedCommentsFromWrite(t *testing.T) {
	args := mustJSON(t, map[string]any{"filePath": "main.go", "content": "package main\n// a\n// b\nvar x = 1\n"})
	if got := addedCommentsFromWrite(args); got != 2 {
		t.Fatalf("write added = %d, want 2", got)
	}
}

func TestAddedCommentsFromEdit(t *testing.T) {
	args := mustJSON(t, map[string]any{
		"filePath":  "main.py",
		"oldString": "# old\nx = 1\n",
		"newString": "# new\nx = 1\n",
	})
	if got := addedCommentsFromEdit(args); got != 0 {
		t.Fatalf("edit net = %d, want 0", got)
	}
	args = mustJSON(t, map[string]any{
		"filePath":  "main.py",
		"oldString": "x = 1\n",
		"newString": "# a\n# b\nx = 1\n",
	})
	if got := addedCommentsFromEdit(args); got != 2 {
		t.Fatalf("edit added = %d, want 2", got)
	}
}

func TestScheduleFire(t *testing.T) {
	s := &agentState{}
	if !scheduleFire(s, 1) {
		t.Fatal("first comment line should fire")
	}
	if scheduleFire(s, 19) {
		t.Fatal("should not fire before 20 more")
	}
	if !scheduleFire(s, 1) {
		t.Fatal("should fire at +20 since last")
	}
	if !scheduleFire(s, 40) {
		t.Fatal("burst of 40 should fire")
	}
	if scheduleFire(s, 5) {
		t.Fatal("should not fire again yet")
	}
	if !scheduleFire(s, 15) {
		t.Fatal("should fire after reaching +20")
	}
}

func newStartedAgent(t *testing.T, bus event.Bus, name string) *agent.Agent {
	t.Helper()
	a := agent.New(name, agent.WithBus(bus))
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID:   a.ID,
		AgentName: a.Name,
		Agent:     a,
	})
	return a
}

func writeCall(agentID, path, content string) agent.AgentToolCall {
	args := `{"filePath":` + strconvQuote(path) + `,"content":` + strconvQuote(content) + `}`
	return agent.AgentToolCall{
		AgentID: agentID,
		Call:    llm.ToolCall{Function: llm.Function{Name: "write", Arguments: args}},
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestReminderFiresOnFirstCommentWrite(t *testing.T) {
	bus := event.New()
	rem := New()
	if err := rem.Start(bus); err != nil {
		t.Fatal(err)
	}
	defer rem.Stop(bus)

	a := newStartedAgent(t, bus, "agent-c")
	if a.HasDisplayNotifications() {
		t.Fatal("no reminder before any tool call")
	}
	bus.Publish(agent.TopicAgentToolCall, writeCall(a.ID, "main.go", "package main\n// hello\n"))
	if !a.HasDisplayNotifications() {
		t.Fatal("expected a reminder after the first comment line")
	}
}

func TestReminderSilentForPlainWrite(t *testing.T) {
	bus := event.New()
	rem := New()
	if err := rem.Start(bus); err != nil {
		t.Fatal(err)
	}
	defer rem.Stop(bus)

	a := newStartedAgent(t, bus, "agent-p")
	bus.Publish(agent.TopicAgentToolCall, writeCall(a.ID, "main.go", "package main\nvar x = 1\n"))
	if a.HasDisplayNotifications() {
		t.Fatal("plain code write must not trigger the comment reminder")
	}
}
