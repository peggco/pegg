package settings

import (
	"context"
	"encoding/json"
	"testing"

	corehook "github.com/peggco/pegg/internal/core/hook"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func withSettingsHookBus(t *testing.T) {
	t.Helper()
	corehook.SetDefault(corehook.NewBus())
	t.Cleanup(corehook.ResetDefault)
}

func addSectionsHook(t *testing.T, sections []any) {
	t.Helper()
	corehook.Default().Registry().AddFilterFor("test", hookSettingsSections, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["sections"] = sections
		return payload, nil
	})
}

func TestHookSettingsTabsInjectedByTab(t *testing.T) {
	withSettingsHookBus(t)
	addSectionsHook(t, []any{
		map[string]any{
			"tab":   "general",
			"title": "My settings",
			"fields": []any{
				map[string]any{"ID": "token", "Label": "Token", "Value": "abc"},
			},
		},
		map[string]any{"title": "Extra"},
	})

	s := New(Deps{})
	if len(s.hookTabs) != 2 {
		t.Fatalf("hook tabs = %d", len(s.hookTabs))
	}
	if s.hookTabs[0].hookTab != "general" {
		t.Fatalf("first hook tab = %q", s.hookTabs[0].hookTab)
	}
	if tab := s.extTabAt(1); tab == nil || !tab.hook {
		t.Fatalf("general hook tab not at index 1")
	}
	if kind, ok := s.kindAt(2); !ok || kind != tabSession {
		t.Fatalf("session not after the injected tab: kind=%v ok=%v", kind, ok)
	}
	if tab := s.extTabAt(len(s.slots) - 1); tab == nil || tab.hookTab != "" {
		t.Fatalf("extras tab missing")
	}
	if len(s.tabLabels()) != len(s.slots) {
		t.Fatalf("labels/slots mismatch")
	}
}

func TestSaveHookValuesDispatchesAction(t *testing.T) {
	withSettingsHookBus(t)
	addSectionsHook(t, []any{
		map[string]any{
			"tab":    "general",
			"title":  "My settings",
			"fields": []any{map[string]any{"ID": "token", "Label": "Token", "Value": "abc"}},
		},
	})
	corehook.Default().Registry().AddFilterFor("test", hookSettingsValues, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		values, _ := payload["values"].(map[string]any)
		if values != nil {
			values["token"] = "filtered"
		}
		return payload, nil
	})

	var saved pkgplugin.SettingsSaveHook
	corehook.Default().Registry().AddActionFor("test", hookSettingsSave, 10, func(ctx context.Context, value any) (any, error) {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return nil, json.Unmarshal(raw, &saved)
	})

	s := New(Deps{})
	if len(s.hookTabs) == 0 {
		t.Fatal("no hook tabs")
	}
	tab := s.hookTabs[0]
	tab.settingsValues["token"] = "abc"
	s.saveHookValues(tab)

	if saved.Tab != "general" {
		t.Fatalf("saved tab = %q", saved.Tab)
	}
	if saved.Values["token"] != "filtered" {
		t.Fatalf("saved values = %#v", saved.Values)
	}
	if s.noticeMsg != "Saved" {
		t.Fatalf("notice = %q", s.noticeMsg)
	}
}

func TestHookTabKeyNavigationKeepsBuiltinMapping(t *testing.T) {
	withSettingsHookBus(t)
	addSectionsHook(t, []any{
		map[string]any{"tab": "general", "title": "General plugins"},
	})
	s := New(Deps{})
	s.nextTab() // general -> injected
	if s.tab != 1 {
		t.Fatalf("index = %v", s.tab)
	}
	s.nextTab() // injected -> session
	kind, ok := s.kindAt(int(s.tab))
	if !ok || kind != tabSession {
		t.Fatalf("kind = %v ok=%v", kind, ok)
	}
}

func TestHookSettingsTabDraws(t *testing.T) {
	withSettingsHookBus(t)
	addSectionsHook(t, []any{
		map[string]any{
			"tab":    "general",
			"title":  "My settings",
			"fields": []any{map[string]any{"ID": "token", "Label": "Token", "Value": "abc"}},
		},
	})
	s := New(Deps{})
	if len(s.hookTabs) == 0 {
		t.Fatal("no hook tabs")
	}
	drawSettings(t, 100, 30, s)
	s.tab = 1
	s.tabs.SetActive(1)
	drawSettings(t, 100, 30, s)
}
