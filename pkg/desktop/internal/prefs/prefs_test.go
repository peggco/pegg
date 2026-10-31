package prefs

import "testing"

func TestDefaults(t *testing.T) {
	p := Defaults()
	if p.Theme != "dark" {
		t.Fatalf("default theme = %q, want dark", p.Theme)
	}
	if p.ThemeMode != ThemeModeAuto {
		t.Fatalf("default mode = %q", p.ThemeMode)
	}
	if !p.SendOnEnter || !p.AutoDownload {
		t.Fatal("expected sensible defaults")
	}
}
