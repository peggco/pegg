//go:build gui

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin"
)

func TestBuildPluginSettingsMergesRuntimeStatus(t *testing.T) {
	globalDir := t.TempDir()
	pluginDir := filepath.Join(globalDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kanban", "kanban-github", "broken"} {
		if err := os.WriteFile(filepath.Join(pluginDir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.DefaultConfig()
	runtime := map[string]plugin.RuntimeStatus{
		"kanban-github": {
			Name:   "kanban-github",
			Loaded: true,
			Hooks: []plugin.PluginHookStatus{
				{Name: "kanban.providers", Kind: "filter", Priority: 10},
				{Name: "kanban.card.succeeded", Kind: "action", Priority: 5},
			},
		},
		"broken": {
			Name:  "broken",
			Error: "boot plugin broken: missing token",
		},
	}

	settings := buildPluginSettings(cfg, globalDir, runtime)
	if len(settings) != 3 {
		t.Fatalf("settings = %#v", settings)
	}
	byName := map[string]PluginSetting{}
	for _, setting := range settings {
		byName[setting.Name] = setting
	}

	github, ok := byName["kanban-github"]
	if !ok {
		t.Fatal("kanban-github missing")
	}
	if github.Runtime != "loaded" {
		t.Fatalf("runtime = %q", github.Runtime)
	}
	if len(github.Hooks) != 2 || github.Hooks[0] != "kanban.providers" {
		t.Fatalf("hooks = %#v", github.Hooks)
	}

	failed, ok := byName["broken"]
	if !ok {
		t.Fatal("broken missing")
	}
	if failed.Runtime != "failed" || failed.Error == "" {
		t.Fatalf("failed = %#v", failed)
	}

	kanban, ok := byName["kanban"]
	if !ok {
		t.Fatal("kanban missing")
	}
	if kanban.Runtime != "" {
		t.Fatalf("kanban runtime = %q", kanban.Runtime)
	}
}
