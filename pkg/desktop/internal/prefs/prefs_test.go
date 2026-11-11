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
	if p.SidebarWidth != DefaultSidebarWidth {
		t.Fatalf("default sidebar width = %d, want %d", p.SidebarWidth, DefaultSidebarWidth)
	}
}

func TestMergeSidebarWidth(t *testing.T) {
	if got := merge(Prefs{}).SidebarWidth; got != DefaultSidebarWidth {
		t.Fatalf("missing width = %d, want %d", got, DefaultSidebarWidth)
	}
	if got := merge(Prefs{SidebarWidth: 320}).SidebarWidth; got != 320 {
		t.Fatalf("valid width = %d, want 320", got)
	}
	if got := merge(Prefs{SidebarWidth: 9999}).SidebarWidth; got != DefaultSidebarWidth {
		t.Fatalf("out-of-range width = %d, want %d", got, DefaultSidebarWidth)
	}
}
