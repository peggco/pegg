package acp

import (
	"context"
	"encoding/json"
	"testing"

	jsonpkg "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/plugin"
)

type fakePluginRuntime struct {
	plugins []plugin.RuntimeStatus
	hooks   []plugin.PluginHookSpec
}

func (f *fakePluginRuntime) RuntimeStatus() []plugin.RuntimeStatus { return f.plugins }
func (f *fakePluginRuntime) HookSpecs() []plugin.PluginHookSpec    { return f.hooks }

func TestPluginsRuntimeReportsManagerState(t *testing.T) {
	runtime := &fakePluginRuntime{
		plugins: []plugin.RuntimeStatus{{
			Name:   "kanban-github",
			Path:   "/plugins/kanban-github",
			Loaded: true,
			Hooks:  []plugin.PluginHookStatus{{Name: "kanban.providers", Kind: "filter", Priority: 10}},
		}},
		hooks: []plugin.PluginHookSpec{{
			Plugin: "kanban-github", Name: "kanban.providers", Kind: "filter", Priority: 10,
		}},
	}
	srv := New(nil, nil, nil, nil, nil, nil, nil, nil)
	srv.SetPluginRuntime(runtime)

	raw, err := srv.handlePluginsRuntime(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := jsonpkg.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var result PluginRuntimeResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Plugins) != 1 || result.Plugins[0].Name != "kanban-github" || !result.Plugins[0].Loaded {
		t.Fatalf("plugins = %#v", result.Plugins)
	}
	if len(result.Hooks) != 1 || result.Hooks[0].Name != "kanban.providers" || result.Hooks[0].Plugin != "kanban-github" {
		t.Fatalf("hooks = %#v", result.Hooks)
	}
}

func TestPluginsRuntimeWithoutManagerIsEmpty(t *testing.T) {
	srv := New(nil, nil, nil, nil, nil, nil, nil, nil)
	raw, err := srv.handlePluginsRuntime(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(PluginRuntimeResult)
	if !ok {
		t.Fatalf("result = %#v", raw)
	}
	if len(result.Plugins) != 0 || len(result.Hooks) != 0 {
		t.Fatalf("result = %#v", result)
	}
}
