package plugin

import (
	"errors"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/core/logger"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func newStatusTestManager(t *testing.T) *Manager {
	t.Helper()
	corehook.ResetDefault()
	t.Cleanup(corehook.ResetDefault)
	return NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
}

func TestRuntimeStatusReportsLoadedPluginsAndFailures(t *testing.T) {
	mgr := newStatusTestManager(t)

	instance := &PluginInstance{
		Name:         "kanban-github",
		Path:         "/plugins/kanban-github",
		Version:      "1.2.3",
		Capabilities: []string{"hooks"},
		Hooks: normalizeHookSpecs([]pkgplugin.HookSpec{
			{Name: "kanban.providers", Kind: pkgplugin.HookFilter, Priority: 10},
			{Name: "kanban.card.succeeded", Kind: pkgplugin.HookAction, Priority: 5},
		}),
	}
	mgr.plugins[instance.Name] = instance
	mgr.registerHooks(instance)

	mgr.recordFailure("/plugins/broken", errors.New("boot plugin broken: boom"))

	statuses := mgr.RuntimeStatus()
	if len(statuses) != 2 {
		t.Fatalf("statuses = %#v", statuses)
	}
	byName := map[string]RuntimeStatus{}
	for _, status := range statuses {
		byName[status.Name] = status
	}

	loaded, ok := byName["kanban-github"]
	if !ok || !loaded.Loaded {
		t.Fatalf("loaded status = %#v", loaded)
	}
	if loaded.Version != "1.2.3" || loaded.Path != "/plugins/kanban-github" {
		t.Fatalf("loaded metadata = %#v", loaded)
	}
	if len(loaded.Hooks) != 2 {
		t.Fatalf("hooks = %#v", loaded.Hooks)
	}
	if loaded.Hooks[0].Name != "kanban.card.succeeded" || loaded.Hooks[0].Kind != "action" {
		t.Fatalf("first hook = %#v", loaded.Hooks[0])
	}
	if loaded.Hooks[1].Name != "kanban.providers" || loaded.Hooks[1].Kind != "filter" {
		t.Fatalf("second hook = %#v", loaded.Hooks[1])
	}

	failed, ok := byName["broken"]
	if !ok || failed.Loaded || failed.Error == "" {
		t.Fatalf("failure status = %#v", failed)
	}
	if failed.Path != "/plugins/broken" {
		t.Fatalf("failure path = %q", failed.Path)
	}

	failures := mgr.LoadFailures()
	if len(failures) != 1 || failures[0].Name != "broken" {
		t.Fatalf("failures = %#v", failures)
	}
}

func TestRuntimeStatusToolAndSkillNames(t *testing.T) {
	mgr := newStatusTestManager(t)
	mgr.plugins["browser-use"] = &PluginInstance{
		Name: "browser-use",
		Path: "/plugins/browser-use",
		Tools: []pkgplugin.ToolSpec{
			{Name: "browser_navigate"},
			{Name: "browser_click"},
		},
		Skills: []pkgplugin.SkillSpec{{Name: "browser-use"}},
	}

	statuses := mgr.RuntimeStatus()
	if len(statuses) != 1 {
		t.Fatalf("statuses = %#v", statuses)
	}
	got := statuses[0]
	if len(got.Tools) != 2 || got.Tools[0] != "browser_click" || got.Tools[1] != "browser_navigate" {
		t.Fatalf("tools = %#v", got.Tools)
	}
	if len(got.Skills) != 1 || got.Skills[0] != "browser-use" {
		t.Fatalf("skills = %#v", got.Skills)
	}
}

func TestPluginCmdSetsWorkspaceDir(t *testing.T) {
	cmd := pluginCmd("/bin/true", "/tmp/workspace")
	if cmd.Dir != "/tmp/workspace" {
		t.Fatalf("dir = %q", cmd.Dir)
	}
	if cmd := pluginCmd("/bin/true", ""); cmd.Dir != "" {
		t.Fatalf("empty dir = %q", cmd.Dir)
	}
}
