package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin"
)

func TestPluginsTabCanReincludeExcludedPlugin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir, err := plugin.GetPluginDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := config.ExcludePlugin("beta"); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := New(Deps{Config: cfg})
	tab := s.plugins

	tab.list.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))

	item, ok := tab.list.Selected()
	if !ok {
		t.Fatal("expected beta in plugin list")
	}
	if item.Disabled {
		t.Fatal("excluded plugin row must remain selectable")
	}
	if item.Label != "beta" {
		t.Fatalf("label = %q, want beta", item.Label)
	}

	tab.list.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	excluded, err := config.PluginExcluded("beta")
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Fatal("plugin should be included after toggling")
	}

	item, ok = tab.list.Selected()
	if !ok {
		t.Fatal("expected beta in plugin list after toggle")
	}
	if item.Disabled {
		t.Fatal("included plugin row must remain selectable")
	}
	if item.Label != "beta" {
		t.Fatalf("selection moved to %q after include, want beta", item.Label)
	}

	tab.list.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	excluded, err = config.PluginExcluded("beta")
	if err != nil {
		t.Fatal(err)
	}
	if !excluded {
		t.Fatal("second toggle should exclude the plugin again")
	}

	item, ok = tab.list.Selected()
	if !ok || item.Label != "beta" {
		t.Fatalf("selection moved to %q after exclude, want beta", item.Label)
	}
}
