package home

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func TestHomeViewChromeDrawSmoke(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	page := New()
	page.SetViewChrome(
		[]pkgplugin.ViewLine{{Text: "header line"}},
		pkgplugin.FooterHook{Hint: "footer hint", Lines: []pkgplugin.ViewLine{{Text: "footer line"}}},
	)
	page.SetEmptyFragments([]pkgplugin.ViewFragment{{
		ID:    "card",
		Title: "Plugin",
		Lines: []pkgplugin.ViewLine{{Text: "empty state card"}},
	}})
	page.SetPromptPlaceholder("type something")
	page.Draw(screen, layout.Region{Left: 0, Top: 0, Width: 100, Height: 30}, true)
	screen.Show()
}
