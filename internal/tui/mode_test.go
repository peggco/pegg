package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/page/home"
)

func TestCycleModeOrder(t *testing.T) {
	a := &App{mode: agent.ModeAuto}
	for _, want := range []agent.Mode{agent.ModePlan, agent.ModeBuild, agent.ModeAuto, agent.ModePlan} {
		a.cycleMode()
		if a.mode != want {
			t.Fatalf("cycleMode() mode = %q, want %q", a.mode, want)
		}
	}
}

func TestCycleModeSkippedWhileRunning(t *testing.T) {
	a := &App{mode: agent.ModeAuto, running: true}
	a.cycleMode()
	if a.mode != agent.ModeAuto {
		t.Fatalf("mode changed while running: %q", a.mode)
	}
}

func TestModeTabKey(t *testing.T) {
	a := &App{}
	if !a.modeTab(tcell.NewEventKey(tcell.KeyTab, 0, 0)) {
		t.Fatal("Tab should cycle the mode")
	}
	if a.modeTab(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift)) {
		t.Fatal("Shift+Tab must toggle focus, not cycle the mode")
	}
	if a.modeTab(tcell.NewEventKey(tcell.KeyBacktab, 0, 0)) {
		t.Fatal("Backtab must toggle focus, not cycle the mode")
	}
}

func TestModeTabDeferredToAskPicker(t *testing.T) {
	p := home.New()
	p.SetAsk([]components.AskQuestion{{ID: "q", Question: "Q?", Type: "text"}})
	a := &App{home: p}
	if a.modeTab(tcell.NewEventKey(tcell.KeyTab, 0, 0)) {
		t.Fatal("the ask picker should keep Tab for question navigation")
	}
}
