//go:build gui

package main

import (
	"context"
	"testing"

	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/pkg/desktop/internal/prefs"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func newViewTestApp(t *testing.T) *App {
	t.Helper()
	corehook.SetDefault(corehook.NewBus())
	t.Cleanup(corehook.ResetDefault)
	t.Setenv("HOME", t.TempDir())
	store, err := prefs.Load()
	if err != nil {
		t.Fatal(err)
	}
	app := &App{
		prefs:    store,
		sessions: map[string]*SessionInfo{},
		pending:  map[string]pendingRequest{},
	}
	app.viewHooks = newDesktopViewHooks(app)
	return app
}

func TestDesktopViewHookPopulatesState(t *testing.T) {
	app := newViewTestApp(t)
	corehook.Default().Registry().AddFilterFor("test", HookView, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		view, _ := payload["view"].(map[string]any)
		if view == nil {
			view = map[string]any{}
			payload["view"] = view
		}
		view["header"] = []any{map[string]any{"text": "hello"}}
		view["status_left"] = []any{map[string]any{"text": "left", "style": "accent"}}
		view["settings"] = []any{map[string]any{
			"tab":   "general",
			"title": "My settings",
			"fields": []any{
				map[string]any{"ID": "token", "Label": "Token", "Value": "abc"},
			},
		}}
		return payload, nil
	})

	app.viewHooks.refresh(context.Background())
	state := app.ViewState()
	if len(state.Header) != 1 || state.Header[0].Text != "hello" {
		t.Fatalf("header = %#v", state.Header)
	}
	if len(state.StatusLeft) != 1 || state.StatusLeft[0].Text != "left" {
		t.Fatalf("status = %#v", state.StatusLeft)
	}
	if len(state.SettingsPages) != 1 {
		t.Fatalf("settings pages = %d", len(state.SettingsPages))
	}
	page := state.SettingsPages[0]
	if page.Owner != desktopViewOwner || page.ID != "view:general" {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Settings) != 1 || page.Settings[0].ID != "token" {
		t.Fatalf("fields = %#v", page.Settings)
	}
}

func TestDesktopViewSettingsSaveAndCommand(t *testing.T) {
	app := newViewTestApp(t)
	var saved pkgplugin.SettingsSaveHook
	corehook.Default().Registry().AddActionFor("test", HookViewSettings, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		saved.Tab, _ = payload["tab"].(string)
		values, _ := payload["values"].(map[string]any)
		if values != nil {
			saved.Values = map[string]string{"token": values["token"].(string)}
		}
		return nil, nil
	})
	var commands []string
	corehook.Default().Registry().AddActionFor("test", HookViewCommand, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		commands = append(commands, payload["id"].(string))
		return nil, nil
	})

	if err := app.SaveViewSettings("general", map[string]string{"token": "xyz"}); err != nil {
		t.Fatal(err)
	}
	if saved.Tab != "general" || saved.Values["token"] != "xyz" {
		t.Fatalf("saved = %#v", saved)
	}
	if err := app.RunViewCommand("my-plugin:toggle"); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0] != "my-plugin:toggle" {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestViewSettingsPagesGroupByTab(t *testing.T) {
	pages := viewSettingsPages([]pkgplugin.ViewSection{
		{Tab: "system", Title: "One", Fields: []pkgplugin.TUIField{{ID: "a"}}},
		{Title: "Two", Fields: []pkgplugin.TUIField{{ID: "b"}}},
		{Tab: "system", Title: "Three", Fields: []pkgplugin.TUIField{{ID: "c"}}},
	})
	if len(pages) != 2 {
		t.Fatalf("pages = %d", len(pages))
	}
	if pages[0].ID != "view:system" || len(pages[0].Settings) != 2 {
		t.Fatalf("first = %#v", pages[0])
	}
	if pages[1].ID != "view:extensions" || pages[1].Title != "Extensions" {
		t.Fatalf("second = %#v", pages[1])
	}
}
