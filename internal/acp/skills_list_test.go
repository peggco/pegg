package acp

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/skill"
)

func TestHandleSkillsListIncludesDynamicSkills(t *testing.T) {
	const name = "plugin-list-test"
	if err := skill.RegisterDynamic(&skill.Skill{
		Name:        name,
		Description: "from a plugin",
		Source:      "plugin:test",
		Path:        "/tmp/plugin-test",
		WhenToUse:   "testing",
	}); err != nil {
		t.Fatalf("register dynamic skill: %v", err)
	}

	server := &Server{}
	raw, err := server.handleSkillsList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleSkillsList: %v", err)
	}
	res, ok := raw.(skillsListResult)
	if !ok {
		t.Fatalf("unexpected result type %T", raw)
	}
	for _, item := range res.Skills {
		if item.Name != name {
			continue
		}
		if item.Source != "plugin:test" {
			t.Fatalf("source = %q, want plugin:test", item.Source)
		}
		if item.Path != "/tmp/plugin-test" {
			t.Fatalf("path = %q, want /tmp/plugin-test", item.Path)
		}
		return
	}
	t.Fatalf("dynamic skill %q not listed", name)
}
