package acp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin/install"
)

func writeTestPlugin(t *testing.T, name, version, source string) string {
	t.Helper()
	dir, err := config.GetConfigPath("plugins")
	if err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	if err := install.WriteMetadata(dir, &install.Metadata{
		Name:        name,
		Source:      source,
		Tag:         "v" + version,
		Version:     version,
		InstalledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	return path
}

func TestHandlePluginsList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeTestPlugin(t, "alpha", "1.2.3", "peggco/alpha")
	if _, err := config.ExcludePlugin("alpha"); err != nil {
		t.Fatalf("exclude plugin: %v", err)
	}

	server := &Server{}
	raw, err := server.handlePluginsList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handlePluginsList: %v", err)
	}
	res, ok := raw.(pluginListResult)
	if !ok {
		t.Fatalf("unexpected result type %T", raw)
	}
	if !res.PluginsEnabled {
		t.Fatal("expected plugins to be enabled globally")
	}
	if len(res.Plugins) != 1 {
		t.Fatalf("plugins = %d, want 1", len(res.Plugins))
	}
	plugin := res.Plugins[0]
	if plugin.Name != "alpha" || plugin.Version != "1.2.3" || plugin.Source != "peggco/alpha" {
		t.Fatalf("unexpected plugin: %+v", plugin)
	}
	if plugin.Enabled {
		t.Fatal("expected alpha to be disabled")
	}
	if plugin.Loaded {
		t.Fatal("expected alpha not to be loaded")
	}
}

func TestHandlePluginsToggle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeTestPlugin(t, "alpha", "1.0.0", "peggco/alpha")

	server := &Server{}
	if _, err := server.handlePluginsToggle(context.Background(), json.RawMessage(`{"name":"alpha","enabled":false}`)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	excluded, err := config.PluginExcluded("alpha")
	if err != nil {
		t.Fatalf("excluded: %v", err)
	}
	if !excluded {
		t.Fatal("expected alpha to be excluded")
	}
	if _, err := server.handlePluginsToggle(context.Background(), json.RawMessage(`{"name":"alpha","enabled":true}`)); err != nil {
		t.Fatalf("enable: %v", err)
	}
	excluded, err = config.PluginExcluded("alpha")
	if err != nil {
		t.Fatalf("excluded: %v", err)
	}
	if excluded {
		t.Fatal("expected alpha to be included")
	}
}

func TestHandlePluginsEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeTestPlugin(t, "alpha", "1.0.0", "peggco/alpha")

	server := &Server{}
	params := json.RawMessage(`{"name":"alpha","env":{"ALPHA_TOKEN":"secret"}}`)
	if _, err := server.handlePluginsEnv(context.Background(), params); err != nil {
		t.Fatalf("set env: %v", err)
	}
	values, err := config.PluginOptions("alpha")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if values["ALPHA_TOKEN"] != "secret" {
		t.Fatalf("token = %q, want secret", values["ALPHA_TOKEN"])
	}

	raw, err := server.handlePluginsList(context.Background(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	res := raw.(pluginListResult)
	if len(res.Plugins) != 1 || len(res.Plugins[0].EnvKeys) != 1 || res.Plugins[0].EnvKeys[0] != "ALPHA_TOKEN" {
		t.Fatalf("unexpected env keys: %+v", res.Plugins)
	}

	bad := json.RawMessage(`{"name":"alpha","env":{"BAD-KEY":"x"}}`)
	if _, err := server.handlePluginsEnv(context.Background(), bad); err == nil {
		t.Fatal("expected invalid env key error")
	}
}

func TestHandlePluginsRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := writeTestPlugin(t, "alpha", "1.0.0", "peggco/alpha")

	server := &Server{}
	if _, err := server.handlePluginsRemove(context.Background(), json.RawMessage(`{"name":"alpha"}`)); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("plugin binary still exists: %v", err)
	}
}
