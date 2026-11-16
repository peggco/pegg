package config

import "testing"

func TestExcludeIncludePlugins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	changed, err := ExcludePlugin("alpha", "beta", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	excluded, err := PluginExcluded("alpha")
	if err != nil || !excluded {
		t.Fatalf("alpha excluded: %v %v", excluded, err)
	}
	changed, err = ExcludePlugin("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("duplicate exclusion changed config")
	}
	changed, err = IncludePlugin("alpha")
	if err != nil || !changed {
		t.Fatalf("include: %v %v", changed, err)
	}
	excluded, err = PluginExcluded("alpha")
	if err != nil || excluded {
		t.Fatalf("alpha still excluded: %v %v", excluded, err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Plugins.Exclude) != 1 || cfg.Plugins.Exclude[0] != "beta" {
		t.Fatalf("exclude: %#v", cfg.Plugins.Exclude)
	}
}

func TestSetPluginEnvPersistsAndRemoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := SetPluginEnv("kanban", map[string]string{"TOKEN": "secret"}); err != nil {
		t.Fatal(err)
	}
	env, ok := PluginEnv("kanban")
	if !ok || env["TOKEN"] != "secret" {
		t.Fatalf("env = %#v, ok = %v", env, ok)
	}
	env["TOKEN"] = "mutated"
	reloaded, ok := PluginEnv("kanban")
	if !ok || reloaded["TOKEN"] != "secret" {
		t.Fatalf("env was not copied: %#v", reloaded)
	}

	if err := SetPluginEnv("kanban", map[string]string{"TOKEN": "updated", "ORG": "pegg"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Plugins.Env["kanban"]; got["TOKEN"] != "updated" || got["ORG"] != "pegg" {
		t.Fatalf("stored env = %#v", got)
	}

	if err := SetPluginEnv("kanban", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := PluginEnv("kanban"); ok {
		t.Fatal("expected env to be removed")
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugins.Env != nil {
		t.Fatalf("env = %#v, want nil", cfg.Plugins.Env)
	}
}

func TestSetPluginEnvValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := []struct {
		name  string
		key   string
		value string
	}{
		{name: "kanban", key: "bad key", value: "x"},
		{name: "kanban", key: "1TOKEN", value: "x"},
		{name: "kanban", key: "", value: "x"},
		{name: "../escape", key: "TOKEN", value: "x"},
	}
	for _, tc := range cases {
		if err := SetPluginEnv(tc.name, map[string]string{tc.key: tc.value}); err == nil {
			t.Fatalf("SetPluginEnv(%q, %q) succeeded, want error", tc.name, tc.key)
		}
	}
}
