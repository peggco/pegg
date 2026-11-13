package tools

import (
	"context"
	"sync"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
)

var reg = tool.NewRegistry()

var (
	dynamicReadyMu sync.RWMutex
	dynamicReady   func(context.Context) error
)

func Register(t tool.Tool) error {
	return reg.Register(t)
}

func Unregister(name string) bool {
	return reg.Unregister(name)
}

func Get(name string) (tool.Tool, bool) {
	return reg.Get(name)
}

func List() []tool.Tool {
	return reg.List()
}

func Names() []string {
	return reg.Names()
}

func LLMTools() []llm.Tool {
	return reg.LLMTools()
}

func SetDynamicReady(fn func(context.Context) error) {
	dynamicReadyMu.Lock()
	dynamicReady = fn
	dynamicReadyMu.Unlock()
}

func WaitDynamicReady(ctx context.Context) error {
	dynamicReadyMu.RLock()
	fn := dynamicReady
	dynamicReadyMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
