//go:build gui

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestProviderDriversIncludesBuiltins(t *testing.T) {
	drivers := (&App{}).ProviderDrivers()
	for _, want := range []string{"claude", "gemini", "openai"} {
		if !slices.Contains(drivers, want) {
			t.Fatalf("drivers %v missing %q", drivers, want)
		}
	}
}

func TestAddCustomProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := &App{}
	if err := a.AddCustomProvider("my-gateway", "openai", "sk-test", "https://example.com/v1"); err != nil {
		t.Fatalf("add custom provider: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("providers = %+v", cfg.Providers)
	}
	provider := cfg.Providers[0]
	if provider.Provider != "my-gateway" || provider.Driver != "openai" {
		t.Fatalf("provider = %+v", provider)
	}
	if provider.APIKey != "sk-test" || provider.BaseURL != "https://example.com/v1" {
		t.Fatalf("provider credentials = %+v", provider)
	}
}

func TestAddCustomProviderValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := &App{}
	cases := []struct {
		name    string
		driver  string
		apiKey  string
		baseURL string
		wantErr bool
	}{
		{name: "", driver: "openai", apiKey: "k", baseURL: "https://example.com/v1", wantErr: true},
		{name: "openai", driver: "openai", apiKey: "k", baseURL: "https://example.com/v1", wantErr: true},
		{name: "my-gateway", driver: "missing", apiKey: "k", baseURL: "https://example.com/v1", wantErr: true},
		{name: "my-gateway", driver: "openai", apiKey: "k", baseURL: "", wantErr: true},
		{name: "my-gateway", driver: "openai", apiKey: "", baseURL: "http://localhost:1234/v1", wantErr: false},
	}
	for _, tc := range cases {
		err := a.AddCustomProvider(tc.name, tc.driver, tc.apiKey, tc.baseURL)
		if tc.wantErr && err == nil {
			t.Fatalf("AddCustomProvider(%q, %q, %q, %q) succeeded, want error", tc.name, tc.driver, tc.apiKey, tc.baseURL)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("AddCustomProvider(%q, %q, %q, %q): %v", tc.name, tc.driver, tc.apiKey, tc.baseURL, err)
		}
	}
}

func TestAddCustomProviderPreservesModels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.UpsertProvider(config.LLMConfig{
		Provider: "my-gateway",
		Driver:   "openai",
		Models:   []config.Model{{ID: "local-model"}},
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	a := &App{}
	if err := a.AddCustomProvider("my-gateway", "claude", "sk-new", "https://new.example/v1"); err != nil {
		t.Fatalf("add custom provider: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	provider := cfg.Providers[0]
	if provider.Driver != "claude" || provider.BaseURL != "https://new.example/v1" {
		t.Fatalf("provider = %+v", provider)
	}
	if len(provider.Models) != 1 || provider.Models[0].ID != "local-model" {
		t.Fatalf("models = %+v", provider.Models)
	}
}

func TestBuildPluginSettingsIncludesEnvKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalDir, err := config.GetConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(globalDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "kanban"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Plugins.Env = map[string]map[string]string{
		"kanban": {"TOKEN": "secret", "ORG": "pegg"},
	}
	settings := buildPluginSettings(cfg, globalDir)
	if len(settings) != 1 {
		t.Fatalf("settings = %#v", settings)
	}
	if !slices.Equal(settings[0].Env, []string{"ORG", "TOKEN"}) {
		t.Fatalf("env = %#v", settings[0].Env)
	}
}

func TestSetPluginEnvPersistsWithoutValuesInSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalDir, err := config.GetConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(globalDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "kanban"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.SetPluginEnv("kanban", map[string]string{" TOKEN ": "secret"}); err != nil {
		t.Fatal(err)
	}
	env, err := a.PluginEnv("kanban")
	if err != nil {
		t.Fatal(err)
	}
	if env["TOKEN"] != "secret" {
		t.Fatalf("env = %#v", env)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings := buildPluginSettings(cfg, globalDir)
	if len(settings) != 1 || !slices.Equal(settings[0].Env, []string{"TOKEN"}) {
		t.Fatalf("settings = %#v", settings)
	}

	if err := a.SetPluginEnv("kanban", nil); err != nil {
		t.Fatal(err)
	}
	env, err = a.PluginEnv("kanban")
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Fatalf("env = %#v, want empty", env)
	}
}

func TestSetPluginEnvRejectsInvalidKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := &App{}
	if err := a.SetPluginEnv("kanban", map[string]string{"bad key": "secret"}); err == nil {
		t.Fatal("expected an error for an invalid key")
	}
}
