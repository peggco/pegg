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
