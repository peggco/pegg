package subagent

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent/agents"
)

func TestTaskPromptIncludesAgentDescriptions(t *testing.T) {
	infos := []agents.Info{
		{Name: "developer", Description: "writes code"},
		{Name: "explorer", Description: "reads code"},
		{Name: "nodesc"},
	}
	got, err := generateTaskToolPrompt(infos)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`developer`: writes code",
		"`explorer`: reads code",
		"`nodesc`",
		"subagent_type",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
