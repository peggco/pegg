//go:build gui

package main

import (
	"testing"

	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
)

func TestMergeSkillSettings(t *testing.T) {
	local := []*skill.Skill{
		{Name: "global-skill", Description: "local global", Source: "global", Path: "/home/u/.pegg/skills/global-skill"},
		{Name: "project-skill", Description: "local project", Source: "project", Path: "/ws/.pegg/skills/project-skill"},
	}
	remote := []acpclient.SkillInfo{
		{Name: "global-skill", Description: "agent global", Source: "/home/u/.pegg/skills", Path: "/home/u/.pegg/skills/global-skill"},
		{Name: "project-skill", Description: "agent project", Source: "/ws/.pegg/skills", Path: "/ws/.pegg/skills/project-skill"},
		{Name: "plugin-skill", Description: "from plugin", Source: "plugin:demo", Path: "/home/u/.pegg/plugins/demo"},
	}

	got := mergeSkillSettings(local, remote)
	want := []SkillSetting{
		{Name: "global-skill", Description: "local global", Source: "global", Path: "/home/u/.pegg/skills/global-skill"},
		{Name: "plugin-skill", Description: "from plugin", Source: "plugin:demo", Path: "/home/u/.pegg/plugins/demo"},
		{Name: "project-skill", Description: "local project", Source: "project", Path: "/ws/.pegg/skills/project-skill"},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestMergeSkillSettingsPluginWins(t *testing.T) {
	local := []*skill.Skill{{Name: "shared", Description: "local", Source: "global", Path: "/local"}}
	remote := []acpclient.SkillInfo{{Name: "shared", Description: "plugin", Source: "plugin:demo", Path: "/plugin"}}

	got := mergeSkillSettings(local, remote)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Source != "plugin:demo" || got[0].Description != "plugin" || got[0].Path != "/plugin" {
		t.Fatalf("plugin entry did not win: %#v", got[0])
	}
}

func TestMergeSkillSettingsEmpty(t *testing.T) {
	if got := mergeSkillSettings(nil, nil); len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}
