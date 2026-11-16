package config

import "testing"

func TestNormalizePluginName(t *testing.T) {
	cases := map[string]string{
		"Kanban GitHub":      "kanban-github",
		"kanban_github":      "kanban-github",
		"pegg-kanban-github": "kanban-github",
		"  PEGG--Kanban  ":   "kanban",
		"pegg-browser.use":   "browser-use",
		"":                   "",
	}
	for input, want := range cases {
		if got := NormalizePluginName(input); got != want {
			t.Errorf("NormalizePluginName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPluginConfigEnvForAliases(t *testing.T) {
	cfg := PluginConfig{Env: map[string]map[string]string{
		"Kanban GitHub": {"PEGG_GITHUB_TOKEN": "secret"},
		"pegg-other":    {"OTHER": "value"},
	}}
	env := cfg.EnvFor("pegg-kanban-github")
	if env["PEGG_GITHUB_TOKEN"] != "secret" {
		t.Fatalf("display name alias not matched: %#v", env)
	}
	if _, ok := env["OTHER"]; ok {
		t.Fatalf("unrelated plugin env leaked: %#v", env)
	}
	if got := cfg.EnvFor("Other"); got["OTHER"] != "value" {
		t.Fatalf("normalized alias: %#v", got)
	}
}

func TestSetUnsetPluginEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	changed, err := SetPluginEnv("pegg-demo", map[string]string{"TOKEN": "one", "EMPTY": ""})
	if err != nil || !changed {
		t.Fatalf("set: %v %v", changed, err)
	}
	changed, err = SetPluginEnv("demo", map[string]string{"TOKEN": "two"})
	if err != nil || !changed {
		t.Fatalf("merge: %v %v", changed, err)
	}
	env, err := PluginEnv("Demo")
	if err != nil {
		t.Fatal(err)
	}
	if env["TOKEN"] != "two" || env["EMPTY"] != "" {
		t.Fatalf("env: %#v", env)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Plugins.Env) != 1 {
		t.Fatalf("alias created duplicate entries: %#v", cfg.Plugins.Env)
	}
	changed, err = UnsetPluginEnv("demo", "TOKEN", "MISSING")
	if err != nil || !changed {
		t.Fatalf("unset: %v %v", changed, err)
	}
	env, _ = PluginEnv("demo")
	if _, ok := env["TOKEN"]; ok || env["EMPTY"] != "" {
		t.Fatalf("after unset: %#v", env)
	}
	if _, err := UnsetPluginEnv("demo", "EMPTY"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load()
	if cfg.Plugins.Env != nil {
		t.Fatalf("empty env map not removed: %#v", cfg.Plugins.Env)
	}
}

func TestSetPluginEnvRejectsInvalidKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := SetPluginEnv("demo", map[string]string{"BAD=KEY": "value"}); err == nil {
		t.Fatal("expected invalid key error")
	}
	if _, err := SetPluginEnv(" ", map[string]string{"KEY": "value"}); err == nil {
		t.Fatal("expected missing plugin name error")
	}
}

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
