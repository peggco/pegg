//go:build gui

package main

import (
	"os"
	"testing"

	"github.com/peggco/pegg/internal/tui/ext"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func TestPluginPagesConversion(t *testing.T) {
	ext.Reset()
	defer ext.Reset()

	ext.Register(func() []pkgplugin.TUIPage {
		return []pkgplugin.TUIPage{{
			ID:          "notify",
			Title:       "Notify",
			Description: "Configure channels.",
			Owner:       "notify",
			Sections: []pkgplugin.TUISection{{
				Title: "Channels",
				Items: []pkgplugin.TUIItem{{Title: "slack", Detail: "slack", Badge: "ready"}},
			}},
			Actions: []pkgplugin.TUIAction{{
				ID:    "add",
				Label: "Add channel",
				Fields: []pkgplugin.TUIField{{
					ID: "name", Label: "Name", Required: true,
				}},
			}},
		}}
	})

	pages := pluginPages()
	if len(pages) != 1 {
		t.Fatalf("pages = %#v", pages)
	}
	page := pages[0]
	if page.ID != "notify" || page.Owner != "notify" || page.Title != "Notify" {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Sections) != 1 || len(page.Sections[0].Items) != 1 || page.Sections[0].Items[0].Badge != "ready" {
		t.Fatalf("sections = %#v", page.Sections)
	}
	if len(page.Actions) != 1 || len(page.Actions[0].Fields) != 1 || !page.Actions[0].Fields[0].Required {
		t.Fatalf("actions = %#v", page.Actions)
	}
}

func TestPluginSettingsPagesIntegration(t *testing.T) {
	if os.Getenv("PEGG_PLUGIN_E2E") != "1" {
		t.Skip("set PEGG_PLUGIN_E2E=1 to load installed plugins")
	}

	app := NewApp()
	defer app.resetPluginManager()

	pages, err := app.PluginSettingsPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Skip("no installed plugin exposes a TUI page")
	}

	owners := map[string]bool{}
	for _, page := range pages {
		owners[page.Owner] = true
	}
	if !owners["notify"] || !owners["kanban"] {
		t.Fatalf("expected notify and kanban pages, got %#v", pages)
	}

	result, err := app.PluginSettingsAction("notify", "notify", "reload", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Page == nil || result.Page.ID != "notify" {
		t.Fatalf("reload result = %#v", result)
	}
}
