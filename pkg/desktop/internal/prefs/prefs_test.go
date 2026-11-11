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
	if p.SidebarMode != "sidebar" || p.LayoutMode != "default" || p.Direction != "ltr" {
		t.Fatalf("default layout prefs = %q/%q/%q", p.SidebarMode, p.LayoutMode, p.Direction)
	}
	if p.ChatHistoryMode != "compact" {
		t.Fatalf("default chat history mode = %q", p.ChatHistoryMode)
	}
}

func TestMergeChatHistoryMode(t *testing.T) {
	if got := merge(Prefs{}).ChatHistoryMode; got != "compact" {
		t.Fatalf("missing chat history mode = %q", got)
	}
	if got := merge(Prefs{ChatHistoryMode: "compact"}).ChatHistoryMode; got != "compact" {
		t.Fatalf("valid compact mode = %q", got)
	}
	if got := merge(Prefs{ChatHistoryMode: "detailed"}).ChatHistoryMode; got != "detailed" {
		t.Fatalf("valid detailed mode = %q", got)
	}
	if got := merge(Prefs{ChatHistoryMode: "bogus"}).ChatHistoryMode; got != "compact" {
		t.Fatalf("invalid chat history mode = %q", got)
	}
}

func TestMergeTheme(t *testing.T) {
	cases := map[string]string{
		"":                 "dark",
		"dark":             "dark",
		"light":            "light",
		"pegg-light":       "light",
		"pegg-dark":        "dark",
		"catppuccin-latte": "light",
		"catppuccin-mocha": "dark",
		"tokyonight-day":   "light",
		"rosepine-dawn":    "light",
		"dracula":          "dark",
		"solarized-light":  "light",
		"everforest-dark":  "dark",
	}
	for input, want := range cases {
		if got := merge(Prefs{Theme: input}).Theme; got != want {
			t.Errorf("merge theme %q = %q, want %q", input, got, want)
		}
	}
}

func TestMergeLayoutPrefs(t *testing.T) {
	got := merge(Prefs{SidebarMode: "floating", LayoutMode: "full", Direction: "rtl"})
	if got.SidebarMode != "floating" || got.LayoutMode != "full" || got.Direction != "rtl" {
		t.Fatalf("valid layout prefs = %q/%q/%q", got.SidebarMode, got.LayoutMode, got.Direction)
	}
	fallback := merge(Prefs{SidebarMode: "bogus", LayoutMode: "wide", Direction: "sideways"})
	if fallback.SidebarMode != "sidebar" || fallback.LayoutMode != "default" || fallback.Direction != "ltr" {
		t.Fatalf("invalid layout prefs = %q/%q/%q", fallback.SidebarMode, fallback.LayoutMode, fallback.Direction)
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
