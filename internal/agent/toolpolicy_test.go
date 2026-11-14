package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
)

func TestDisableTools(t *testing.T) {
	a := New("test",
		WithTool(namedTool("keep")),
		WithTool(namedTool("drop")),
	)

	a.DisableTools("drop")
	if !a.IsToolDisabled("drop") {
		t.Fatal("drop should be disabled")
	}
	if a.IsToolDisabled("keep") {
		t.Fatal("keep should not be disabled")
	}
	if got := a.DisabledTools(); len(got) != 1 || got[0] != "drop" {
		t.Fatalf("DisabledTools() = %v, want [drop]", got)
	}

	names := map[string]bool{}
	for _, tl := range a.visibleTools() {
		names[tl.Function.Name] = true
	}
	if names["drop"] {
		t.Fatal("disabled tool must not be visible")
	}
	if !names["keep"] {
		t.Fatal("enabled tool must be visible")
	}

	a.EnableTools("drop")
	if a.IsToolDisabled("drop") {
		t.Fatal("drop should be enabled again")
	}
}

func TestRunToolRejectsDisabledTool(t *testing.T) {
	a := New("test", WithTool(namedTool("drop")))
	a.DisableTools("drop")

	res := a.runTool(context.Background(), &runState{}, llm.ToolCall{
		Function: llm.Function{Name: "drop"},
	})
	if !errors.Is(res.toolErr, tool.ErrDisabled) {
		t.Fatalf("toolErr = %v, want ErrDisabled", res.toolErr)
	}
	if res.output == "" {
		t.Fatal("expected an explanatory output")
	}
}

func TestCloneCopiesDisabledTools(t *testing.T) {
	a := New("parent")
	a.DisableTools("askuserquestion")

	c := a.Clone("child")
	if !c.IsToolDisabled("askuserquestion") {
		t.Fatal("clone should inherit disabled tools")
	}
}
