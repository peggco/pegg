package acp

import (
	"context"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/plugin"
)

type PluginRuntimeResult struct {
	Plugins []plugin.RuntimeStatus  `json:"plugins"`
	Hooks   []plugin.PluginHookSpec `json:"hooks"`
}

func (s *Server) handlePluginsRuntime(_ context.Context, _ json.RawMessage) (any, error) {
	result := PluginRuntimeResult{
		Plugins: []plugin.RuntimeStatus{},
		Hooks:   []plugin.PluginHookSpec{},
	}
	if s.plugins == nil {
		return result, nil
	}
	if plugins := s.plugins.RuntimeStatus(); plugins != nil {
		result.Plugins = plugins
	}
	if hooks := s.plugins.HookSpecs(); hooks != nil {
		result.Hooks = hooks
	}
	return result, nil
}
