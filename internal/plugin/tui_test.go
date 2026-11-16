package plugin

import (
	"context"
	"testing"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/tui/ext"
)

type tuiPlugin struct {
	pkgplugin.Base
	pages  []pkgplugin.TUIPage
	action string
}

func (p *tuiPlugin) Name() string { return "tui-test" }

func (p *tuiPlugin) Capabilities() []string {
	return []string{string(pkgplugin.CapabilityTUI)}
}

func (p *tuiPlugin) TUIPages() ([]pkgplugin.TUIPage, error) {
	return p.pages, nil
}

func (p *tuiPlugin) ExecuteTUIAction(req pkgplugin.TUIActionRequest) (*pkgplugin.TUIActionResult, error) {
	p.action = req.ActionID
	page := pkgplugin.TUIPage{ID: "main", Title: "Updated"}
	return &pkgplugin.TUIActionResult{Output: "ok", Page: &page}, nil
}

func TestTUIPagesCaptureAndExecute(t *testing.T) {
	ext.Reset()
	defer ext.Reset()

	impl := &tuiPlugin{pages: []pkgplugin.TUIPage{{ID: "main", Title: "Main"}}}
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	instance := &PluginInstance{
		Name:         impl.Name(),
		Plugin:       impl,
		Capabilities: impl.Capabilities(),
	}
	mgr.plugins[instance.Name] = instance
	mgr.captureCapabilities(instance)

	if len(instance.TUIPages) != 1 || instance.TUIPages[0].Owner != instance.Name {
		t.Fatalf("cached pages: %#v", instance.TUIPages)
	}
	pages := ext.Pages()
	if len(pages) != 1 || pages[0].ID != "main" || pages[0].Owner != instance.Name {
		t.Fatalf("hook pages: %#v", pages)
	}

	res, err := mgr.ExecuteTUIAction(context.Background(), instance.Name, pkgplugin.TUIActionRequest{PageID: "main", ActionID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "ok" || impl.action != "test" {
		t.Fatalf("result: %#v action=%q", res, impl.action)
	}
	if instance.TUIPages[0].Title != "Updated" || instance.TUIPages[0].Owner != instance.Name {
		t.Fatalf("updated cache: %#v", instance.TUIPages)
	}
}

func TestPagesAndOwners(t *testing.T) {
	impl := &tuiPlugin{pages: []pkgplugin.TUIPage{{ID: "main", Title: "Main"}}}
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	instance := &PluginInstance{
		Name:         impl.Name(),
		Plugin:       impl,
		Capabilities: impl.Capabilities(),
		TUIPages:     []pkgplugin.TUIPage{{ID: "main", Title: "Main", Owner: impl.Name()}},
	}
	mgr.plugins[instance.Name] = instance

	pages := mgr.Pages(instance.Name)
	if len(pages) != 1 || pages[0].ID != "main" {
		t.Fatalf("pages: %#v", pages)
	}
	pages[0].Title = "mutated"
	if mgr.Pages(instance.Name)[0].Title != "Main" {
		t.Fatal("Pages must return a copy")
	}
	if owners := mgr.PageOwners(); len(owners) != 1 || owners[0] != instance.Name {
		t.Fatalf("owners: %#v", owners)
	}
	if got := mgr.Pages("missing"); len(got) != 0 {
		t.Fatalf("expected no pages for unknown owner, got %#v", got)
	}
}
