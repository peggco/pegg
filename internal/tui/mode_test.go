package tui

import (
	"testing"

	"github.com/peggco/pegg/internal/agent"
)

func TestCycleModeOrder(t *testing.T) {
	a := &App{mode: agent.ModeAuto}
	for _, want := range []agent.Mode{agent.ModeBuild, agent.ModePlan, agent.ModeAuto, agent.ModeBuild} {
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
