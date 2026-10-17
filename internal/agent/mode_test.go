package agent

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
)

func TestNormalizeMode(t *testing.T) {
	cases := map[Mode]Mode{
		"":         ModeAuto,
		"nonsense": ModeAuto,
		ModeAuto:   ModeAuto,
		ModePlan:   ModePlan,
		ModeBuild:  ModeBuild,
	}
	for in, want := range cases {
		if got := NormalizeMode(in); got != want {
			t.Fatalf("NormalizeMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetModeAppliesController(t *testing.T) {
	var got []Mode
	a := New("test", WithModeController(func(_ *Agent, m Mode) {
		got = append(got, m)
	}))
	if a.CurrentMode() != ModeAuto {
		t.Fatalf("default mode = %q, want auto", a.CurrentMode())
	}
	a.SetMode(ModePlan)
	a.SetMode("bogus")
	if len(got) != 2 || got[0] != ModePlan || got[1] != ModeAuto {
		t.Fatalf("controller calls = %v, want [plan auto]", got)
	}
}

func namedTool(name string) tool.Tool {
	return tool.NewSpec(name, "test", map[string]any{"type": "object"}, func(_ context.Context, _ string) (string, error) {
		return name, nil
	})
}

func TestVisibleToolsHidesPlanToolsOutsideAuto(t *testing.T) {
	a := New("test",
		WithTool(namedTool("echo")),
		WithTool(namedTool(ToolEnterPlanMode)),
		WithTool(namedTool(ToolExitPlanMode)),
	)

	assertTools := func(mode Mode, wantPlan bool) {
		a.SetMode(mode)
		names := map[string]bool{}
		for _, tl := range a.visibleTools() {
			names[tl.Function.Name] = true
		}
		if !names["echo"] {
			t.Fatalf("mode %q: echo tool missing", mode)
		}
		if names[ToolEnterPlanMode] != wantPlan || names[ToolExitPlanMode] != wantPlan {
			t.Fatalf("mode %q: plan tools present=%v, want %v", mode, names[ToolEnterPlanMode], wantPlan)
		}
	}

	assertTools(ModeAuto, true)
	assertTools(ModeBuild, false)
	assertTools(ModePlan, false)
}

func TestVisibleToolsEmpty(t *testing.T) {
	a := New("test")
	if got := a.visibleTools(); len(got) != 0 {
		t.Fatalf("visibleTools() = %d tools, want 0", len(got))
	}
}
