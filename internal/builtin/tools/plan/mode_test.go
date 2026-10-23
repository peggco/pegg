package plan

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/vfs"
)

func TestApplyModePlanAttachesReminderAndRestrictsWrites(t *testing.T) {
	fs, err := vfs.New(t.TempDir(), vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write("main.go", []byte("package main")); err != nil {
		t.Fatal(err)
	}

	a := agent.New("test-agent")
	ApplyMode(a, fs, agent.ModePlan)

	r := a.StandingReminder()
	if r == nil || r.Tag != "plan_mode" {
		t.Fatalf("plan mode reminder not attached: %+v", r)
	}
	if !strings.Contains(r.Content, "Phase 5: Present the Plan") {
		t.Errorf("reminder = %q, want present-the-plan phase", r.Content)
	}
	if strings.Contains(r.Content, "Phase 5: Call ExitPlanMode") {
		t.Errorf("reminder = %q, must not tell the agent to call exitplanmode", r.Content)
	}
	if _, err := fs.Write(".pegg/plans/example-feature.md", []byte("plan")); err != nil {
		t.Fatalf("write to plans: %v", err)
	}
	if _, err := fs.Write("main.go", []byte("x")); err == nil {
		t.Fatal("write outside plans must fail in plan mode")
	}
}

func TestApplyModeBuildClearsPlanState(t *testing.T) {
	fs, err := vfs.New(t.TempDir(), vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a := agent.New("test-agent")
	ApplyMode(a, fs, agent.ModePlan)

	ApplyMode(a, fs, agent.ModeBuild)

	if r := a.StandingReminder(); r != nil {
		t.Fatalf("reminder still attached after build mode: %+v", r)
	}
	if _, err := fs.Write("main.go", []byte("package main")); err != nil {
		t.Fatalf("write after build mode: %v", err)
	}
}

func TestApplyModeAutoClearsPlanState(t *testing.T) {
	a := agent.New("test-agent")
	ApplyMode(a, nil, agent.ModePlan)
	ApplyMode(a, nil, agent.ModeAuto)
	if r := a.StandingReminder(); r != nil {
		t.Fatalf("reminder still attached after auto mode: %+v", r)
	}
}
