package settings

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/plugin"
	"github.com/peggco/pegg/internal/tui/ext"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func TestSettingsRendersInstalledPluginPages(t *testing.T) {
	if os.Getenv("PEGG_PLUGIN_E2E") != "1" {
		t.Skip("set PEGG_PLUGIN_E2E=1 to render installed plugin TUI pages")
	}

	ext.Reset()
	mgr := plugin.NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	if err := mgr.LoadPlugins(); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	if len(ext.Pages()) == 0 {
		t.Skip("no installed plugin exposes a TUI page")
	}

	styles.RegisterDefaults()
	styles.Set("dark")
	s := New(Deps{Plugin: mgr})

	labels := s.tabLabels()
	notifyIndex := -1
	for i, label := range labels {
		if label == "Notify" {
			notifyIndex = i
			break
		}
	}
	if notifyIndex < 0 {
		t.Fatalf("Notify tab missing from %v", labels)
	}

	s.tab = tabKind(notifyIndex)
	s.tabs.SetActive(notifyIndex)
	s.focus = settingsFocusContent

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	s.Draw(screen, layout.Region{Left: 0, Top: 0, Width: 120, Height: 40}, true)
	screen.Show()

	cells, width, height := screen.GetContents()
	var text strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := cells[y*width+x]
			if len(cell.Runes) > 0 {
				text.WriteRune(cell.Runes[0])
			} else {
				text.WriteRune(' ')
			}
		}
		text.WriteByte('\n')
	}

	body := text.String()
	for _, want := range []string{"Notify", "CHANNELS", "Send a test notification"} {
		if !strings.Contains(body, want) {
			t.Fatalf("screen missing %q:\n%s", want, body)
		}
	}

	res, err := mgr.ExecuteTUIAction(context.Background(), "notify", pkgplugin.TUIActionRequest{
		PageID:   "notify",
		ActionID: "reload",
	})
	if err != nil {
		t.Fatalf("notify reload action: %v", err)
	}
	if res.Page == nil || res.Page.ID != "notify" {
		t.Fatalf("notify reload page: %#v", res.Page)
	}
}
