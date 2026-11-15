package settings

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/ext"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func screenText(s tcell.Screen) string {
	sim, ok := s.(tcell.SimulationScreen)
	if !ok {
		return ""
	}
	cells, w, h := sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			if len(cell.Runes) > 0 {
				b.WriteRune(cell.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func drawExtTab(t *testing.T, s *Settings, w, h int) string {
	t.Helper()
	screen := newSim(t, w, h)
	s.Draw(screen, layout.Region{Left: 0, Top: 0, Width: w, Height: h}, true)
	screen.Show()
	return screenText(screen)
}

func TestExtTabRegistrationAddsTabsAndLabels(t *testing.T) {
	ext.Reset()
	t.Cleanup(ext.Reset)
	ext.Register(func() []ext.Page {
		return []ext.Page{
			{ID: "notify", Title: "Notify", Owner: "notify"},
			{ID: "unnamed", Owner: "anon"},
		}
	})

	s := New(Deps{})
	if got := s.tabs.Count(); got != builtinTabCount+2 {
		t.Fatalf("tab count = %d, want %d", got, builtinTabCount+2)
	}
	labels := s.tabLabels()
	if labels[builtinTabCount] != "Notify" {
		t.Errorf("first ext label = %q, want Notify", labels[builtinTabCount])
	}
	if labels[builtinTabCount+1] != "Plugin" {
		t.Errorf("fallback ext label = %q, want Plugin", labels[builtinTabCount+1])
	}
	if s.extTabs[1].Title() != "Plugin" {
		t.Errorf("fallback title = %q, want Plugin", s.extTabs[1].Title())
	}
}

func TestExtTabsWrapAcrossBuiltins(t *testing.T) {
	ext.Reset()
	t.Cleanup(ext.Reset)
	ext.Register(func() []ext.Page {
		return []ext.Page{{ID: "notify", Title: "Notify"}}
	})

	s := New(Deps{})
	s.tab = tabSystem
	s.tabs.SetActive(int(tabSystem))
	s.nextTab()
	if s.tab != tabKind(builtinTabCount) {
		t.Fatalf("nextTab from System = %d, want %d", s.tab, builtinTabCount)
	}
	s.prevTab()
	if s.tab != tabSystem {
		t.Fatalf("prevTab from ext = %d, want %d", s.tab, tabSystem)
	}
}

func TestExtTabDrawsPage(t *testing.T) {
	ext.Reset()
	t.Cleanup(ext.Reset)
	ext.Register(func() []ext.Page {
		return []ext.Page{{
			ID:          "notify",
			Title:       "Notify",
			Description: "Configure channels",
			Owner:       "notify",
			Metrics:     []pkgplugin.TUIMetric{{Label: "Channels", Value: "2"}},
			Sections: []pkgplugin.TUISection{{
				Title: "channels",
				Items: []pkgplugin.TUIItem{{Title: "slack", Detail: "hooks"}},
			}},
			Actions: []pkgplugin.TUIAction{{ID: "test", Label: "Send test", Description: "send a message"}},
		}}
	})

	styles.RegisterDefaults()
	styles.Set("dark")
	s := New(Deps{})
	s.tab = tabKind(builtinTabCount)
	s.tabs.SetActive(builtinTabCount)
	s.focus = settingsFocusContent

	text := drawExtTab(t, s, 100, 30)
	for _, want := range []string{"Notify", "Configure channels", "CHANNELS", "slack", "ACTIONS", "Send test"} {
		if !strings.Contains(text, want) {
			t.Errorf("screen missing %q:\n%s", want, text)
		}
	}
}

func TestExtTabSkipsBoardSections(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Sections: []pkgplugin.TUISection{
			{Title: "Board", Kind: "board", Items: []pkgplugin.TUIItem{{Title: "card one", Column: "ready"}}},
			{Title: "Info", Kind: "list", Items: []pkgplugin.TUIItem{{Title: "Repo", Detail: "owner/name"}}},
		},
	}
	tab := newExtTab(s, page)
	text := docText(tab.doc)
	if strings.Contains(text, "card one") {
		t.Errorf("board section should be skipped:\n%s", text)
	}
	if !strings.Contains(text, "Repo") {
		t.Errorf("list section missing:\n%s", text)
	}
}

func TestExtTabTilesAndToolbar(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID:      "p",
		Title:   "Kanban",
		Metrics: []pkgplugin.TUIMetric{{Label: "Cards", Value: "1"}, {Label: "GitHub", Value: "ready", Detail: "via pegg-kanban-github"}},
		Actions: []pkgplugin.TUIAction{
			{ID: "start", Label: "Start server"},
			{ID: "open", Label: "Open board"},
			{ID: "stop", Label: "Stop server"},
			{ID: "card.add", Label: "Add card"},
		},
	}
	tab := newExtTab(s, page)
	text := docText(tab.doc)
	for _, want := range []string{"CARDS", "GITHUB", "ready", "Start server", "Open board", "Stop server", "Add card"} {
		if !strings.Contains(text, want) {
			t.Errorf("doc missing %q:\n%s", want, text)
		}
	}
	if len(tab.doc.hits) != 4 {
		t.Errorf("toolbar hits = %d, want 4", len(tab.doc.hits))
	}
}

func TestExtTabSelectionMovesAndActivates(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Actions: []pkgplugin.TUIAction{
			{ID: "start", Label: "Start server"},
			{ID: "stop", Label: "Stop server"},
		},
	}
	tab := newExtTab(s, page)
	var ran []string
	s.extActionRunner = func(_ *extTab, action pkgplugin.TUIAction, _ map[string]string) {
		ran = append(ran, action.ID)
	}

	tab.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if len(ran) != 1 || ran[0] != "start" {
		t.Fatalf("first enter ran %v, want [start]", ran)
	}
	tab.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	tab.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if len(ran) != 2 || ran[1] != "stop" {
		t.Fatalf("second enter ran %v, want [start stop]", ran)
	}
}

func TestExtTabItemMetaActions(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Sections: []pkgplugin.TUISection{{
			Title: "Channels",
			Kind:  "channels",
			Items: []pkgplugin.TUIItem{{
				ID:     "alerts",
				Title:  "alerts",
				Detail: "slack · #general",
				Badge:  "ready",
				Meta:   map[string]string{"testAction": "test", "deleteAction": "remove"},
			}},
		}},
	}
	tab := newExtTab(s, page)
	var ran []string
	var values map[string]string
	s.extActionRunner = func(_ *extTab, action pkgplugin.TUIAction, v map[string]string) {
		ran = append(ran, action.ID)
		values = v
	}

	if !tab.testSelected() {
		t.Fatal("testSelected returned false")
	}
	if len(ran) != 1 || ran[0] != "test" {
		t.Fatalf("test ran %v, want [test]", ran)
	}
	if values["channel"] != "alerts" {
		t.Errorf("test values = %v, want channel=alerts", values)
	}
	if !tab.deleteSelected() {
		t.Fatal("deleteSelected returned false")
	}
	if ran[1] != "remove" || values["name"] != "alerts" {
		t.Errorf("delete ran %v values %v", ran, values)
	}
}

func TestExtTabSelectionHighlight(t *testing.T) {
	ext.Reset()
	t.Cleanup(ext.Reset)
	ext.Register(func() []ext.Page {
		return []ext.Page{{
			ID:    "p",
			Title: "P",
			Sections: []pkgplugin.TUISection{{
				Title: "Channels",
				Kind:  "channels",
				Items: []pkgplugin.TUIItem{{ID: "a", Title: "alpha", Detail: "slack"}},
			}},
		}}
	})
	styles.RegisterDefaults()
	styles.Set("dark")
	s := New(Deps{})
	s.tab = tabKind(builtinTabCount)
	s.tabs.SetActive(builtinTabCount)
	s.focus = settingsFocusContent

	screen := newSim(t, 100, 30)
	s.Draw(screen, layout.Region{Left: 0, Top: 0, Width: 100, Height: 30}, true)
	screen.Show()

	sim := screen.(tcell.SimulationScreen)
	cells, w, h := sim.GetContents()
	th := styles.Current()
	found := false
	for y := 0; y < h && !found; y++ {
		var row strings.Builder
		for x := 0; x < w; x++ {
			if len(cells[y*w+x].Runes) > 0 {
				row.WriteRune(cells[y*w+x].Runes[0])
			} else {
				row.WriteByte(' ')
			}
		}
		idx := strings.Index(row.String(), "alpha")
		if idx < 0 {
			continue
		}
		_, bg, _ := cells[y*w+idx].Style.Decompose()
		if bg != th.Selection {
			t.Errorf("selected row bg = %v, want %v", bg, th.Selection)
		}
		found = true
	}
	if !found {
		t.Fatal("alpha row not found on screen")
	}
}

func TestExtTabInfoRowsNavigable(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Sections: []pkgplugin.TUISection{{
			Title: "Server",
			Kind:  "list",
			Items: []pkgplugin.TUIItem{
				{Title: "Status", Detail: "stopped"},
				{Title: "Repo", Detail: "owner/name", Action: "settings.save", Fields: []pkgplugin.TUIField{{ID: "github_repo", Label: "GitHub repo", Value: "owner/name"}}},
			},
		}},
	}
	tab := newExtTab(s, page)
	if len(tab.doc.hits) != 2 {
		t.Fatalf("hits = %d, want 2 (readonly rows must be navigable)", len(tab.doc.hits))
	}
	tab.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if tab.sel != 1 {
		t.Fatalf("sel = %d, want 1", tab.sel)
	}
	hit := tab.selectedHit()
	if hit == nil || len(hit.fields) != 1 || hit.fields[0].ID != "github_repo" {
		t.Fatalf("selected hit = %#v", hit)
	}
}

func TestExtTabEditableInfoRowOpensForm(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Sections: []pkgplugin.TUISection{{
			Title: "GitHub",
			Kind:  "list",
			Items: []pkgplugin.TUIItem{
				{Title: "Repo", Detail: "owner/name", Action: "settings.save", Fields: []pkgplugin.TUIField{{ID: "github_repo", Label: "GitHub repo", Value: "owner/name"}}},
			},
		}},
	}
	tab := newExtTab(s, page)
	tab.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if !s.HasSub() {
		t.Fatal("expected an edit form to open for an editable info row")
	}
}

func TestExtTabInfoRowOptionCycles(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Sections: []pkgplugin.TUISection{{
			Title: "Activity",
			Kind:  "list",
			Items: []pkgplugin.TUIItem{
				{Title: "Enabled", Detail: "true", Action: "settings.save", Fields: []pkgplugin.TUIField{{ID: "enabled", Label: "Enabled", Value: "true", Options: []string{"true", "false"}}}},
			},
		}},
	}
	tab := newExtTab(s, page)
	var got map[string]string
	s.extActionRunner = func(_ *extTab, _ pkgplugin.TUIAction, values map[string]string) {
		got = values
	}
	tab.HandleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if got["enabled"] != "false" {
		t.Fatalf("cycled values = %v, want enabled=false", got)
	}
}

func TestExtTabChipIsColorless(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	th := styles.Current()
	_, bg, _ := extChipStyle(th).Decompose()
	if bg != th.InputBg {
		t.Errorf("chip bg = %v, want content bg %v", bg, th.InputBg)
	}
}

func TestExtActionPickerThenForm(t *testing.T) {
	s := New(Deps{})
	page := pkgplugin.TUIPage{
		ID: "p",
		Actions: []pkgplugin.TUIAction{{
			ID: "add", Label: "Add channel", PickerField: "type",
			Fields: []pkgplugin.TUIField{
				{ID: "type", Label: "Channel type", Options: []string{"slack", "telegram"}},
				{ID: "name", Label: "Name"},
				{ID: "webhook_url", Label: "Webhook", ShowWhen: &pkgplugin.TUIFieldCondition{Field: "type", Values: []string{"slack"}}},
				{ID: "bot_token", Label: "Bot token", ShowWhen: &pkgplugin.TUIFieldCondition{Field: "type", Values: []string{"telegram"}}},
			},
		}},
	}
	tab := newExtTab(s, page)
	s.beginExtAction(tab, page.Actions[0])

	picker, ok := s.sub.(*extPicker)
	if !ok {
		t.Fatalf("expected picker, got %T", s.sub)
	}
	picker.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	picker.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	form, ok := s.sub.(*extForm)
	if !ok {
		t.Fatalf("expected form after picker, got %T", s.sub)
	}
	ids := []string{}
	for _, f := range form.fields {
		ids = append(ids, f.spec.ID)
	}
	if len(ids) != 2 || ids[0] != "name" || ids[1] != "bot_token" {
		t.Fatalf("telegram form fields = %v, want [name bot_token]", ids)
	}

	var got map[string]string
	s.extActionRunner = func(_ *extTab, _ pkgplugin.TUIAction, values map[string]string) {
		got = values
	}
	form.submit()
	if got["type"] != "telegram" {
		t.Errorf("submitted type = %q, want telegram (values=%v)", got["type"], got)
	}
}

func TestExtTabRefreshWithoutPluginSetsError(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, pkgplugin.TUIPage{ID: "p", Title: "P"})
	tab.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'r', 0))
	if s.errMsg == "" {
		t.Fatal("refresh without a plugin manager should set errMsg")
	}
}

func docText(doc *extDoc) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	for _, row := range doc.rows {
		for _, seg := range row.segs {
			b.WriteString(seg.text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
