package connect

import (
	"testing"

	"github.com/peggco/pegg/internal/agent"
)

func TestResolveRunMode(t *testing.T) {
	cases := []struct {
		name        string
		provided    string
		sessionMode string
		want        agent.Mode
	}{
		{"explicit build", "build", "plan", agent.ModeBuild},
		{"explicit plan", "plan", "build", agent.ModePlan},
		{"explicit auto", "auto", "plan", agent.ModeAuto},
		{"invalid falls back to session", "nonsense", "plan", agent.ModePlan},
		{"empty falls back to session", "", "build", agent.ModeBuild},
		{"empty session defaults to auto", "", "", agent.ModeAuto},
		{"invalid session defaults to auto", "bogus", "also-bogus", agent.ModeAuto},
	}
	for _, tc := range cases {
		if got := resolveRunMode(tc.provided, tc.sessionMode); got != tc.want {
			t.Errorf("%s: resolveRunMode(%q, %q) = %q, want %q", tc.name, tc.provided, tc.sessionMode, got, tc.want)
		}
	}
}
