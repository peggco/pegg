package themes

import "testing"

func TestAllThemesHaveTokens(t *testing.T) {
	all := All()
	if len(all) < 20 {
		t.Fatalf("expected all TUI themes, got %d", len(all))
	}
	seen := map[string]bool{}
	for _, theme := range all {
		if theme.Name == "" || theme.Label == "" {
			t.Errorf("theme missing name/label: %+v", theme)
		}
		if seen[theme.Name] {
			t.Errorf("duplicate theme %q", theme.Name)
		}
		seen[theme.Name] = true
		for _, key := range []string{"--pg-bg", "--pg-surface", "--pg-text", "--pg-accent", "--pg-border", "--pg-code-bg"} {
			if theme.Tokens[key] == "" {
				t.Errorf("theme %q missing %s", theme.Name, key)
			}
		}
	}
	if !seen["dark"] || !seen["light"] {
		t.Fatal("expected dark and light TUI themes")
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("dark"); !ok {
		t.Fatal("dark theme not found")
	}
	if _, ok := Find("pegg-light"); ok {
		t.Fatal("legacy pegg theme should not exist")
	}
}
