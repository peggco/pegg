package tui

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/tui/components"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func TestHeaderLinesIncludesTitle(t *testing.T) {
	lines := headerLines(pkgplugin.HeaderHook{
		Title: "My plugin",
		Lines: []pkgplugin.ViewLine{{Text: "line"}},
	})
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if len(lines[0].Spans) != 1 || lines[0].Spans[0].Text != "My plugin" {
		t.Fatalf("title line = %#v", lines[0])
	}
}

func TestBindingHint(t *testing.T) {
	got := bindingHint([]pkgplugin.ViewKeyBinding{
		{Key: "ctrl+k", Label: "Search"},
		{Key: "f2"},
	})
	if got != "Search · f2" {
		t.Fatalf("hint = %q", got)
	}
}

func TestStatusSpans(t *testing.T) {
	spans := statusSpans(components.StatusSnapshot{Mode: "auto", ModelName: "gpt", Provider: "openai"})
	if len(spans) != 3 || spans[0].Text != "auto" || spans[2].Text != "openai" {
		t.Fatalf("spans = %#v", spans)
	}
}

func TestDecorateMessage(t *testing.T) {
	corehook.SetDefault(corehook.NewBus())
	t.Cleanup(corehook.ResetDefault)
	corehook.Default().Registry().AddFilterFor("test", HookMessageDecorate, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["prefix"] = "[tag] "
		return payload, nil
	})
	app := &App{}
	got := app.decorateMessage(agent.AgentMessage{AgentID: "a", AgentName: "dev", Message: llm.AssistantMessage("hi")}, "hello")
	if got != "[tag] hello" {
		t.Fatalf("decorated = %q", got)
	}
}

func TestSyncPluginChatItems(t *testing.T) {
	app := &App{chat: components.NewChat(), viewID: "main"}
	app.main = newTranscript("main", "main")
	normal := &components.ChatItem{Kind: components.ItemUser, ID: "user-1", Text: "hello"}
	app.main.items = []*components.ChatItem{normal}
	app.chat.SetItems(app.main.items)

	app.syncPluginChatItems([]pkgplugin.ViewFragment{
		{ID: "b", Title: "B", Priority: 20, Lines: []pkgplugin.ViewLine{{Text: "second"}}},
		{ID: "a", Title: "A", Priority: 10, Lines: []pkgplugin.ViewLine{{Text: "first"}}},
	})
	if len(app.main.items) != 3 {
		t.Fatalf("items = %d", len(app.main.items))
	}
	if app.main.items[0].ID != "user-1" || app.main.items[1].ID != "plugin:a" || app.main.items[2].ID != "plugin:b" {
		t.Fatalf("order = %s, %s, %s", app.main.items[0].ID, app.main.items[1].ID, app.main.items[2].ID)
	}
	if app.main.items[1].Kind != components.ItemNotice {
		t.Fatalf("kind = %v", app.main.items[1].Kind)
	}

	app.syncPluginChatItems([]pkgplugin.ViewFragment{
		{ID: "a", Title: "A", Priority: 10, Lines: []pkgplugin.ViewLine{{Text: "first"}}},
	})
	if len(app.main.items) != 2 {
		t.Fatalf("items after update = %d", len(app.main.items))
	}
	if app.main.items[1].ID != "plugin:a" {
		t.Fatalf("remaining = %s", app.main.items[1].ID)
	}
}
