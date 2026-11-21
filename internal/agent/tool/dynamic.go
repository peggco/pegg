package tool

import (
	"context"
	"sync"
)

var (
	dynamicReadyMu sync.RWMutex
	dynamicReady   func(context.Context) error
)

// SetDynamicReady registers a hook that reports when dynamically discovered
// tools (for example MCP servers) are ready in the default catalogue.
func SetDynamicReady(fn func(context.Context) error) {
	dynamicReadyMu.Lock()
	dynamicReady = fn
	dynamicReadyMu.Unlock()
}

// WaitDynamicReady blocks until the hook registered by SetDynamicReady reports
// readiness or the context is cancelled. It is a no-op when no hook is set.
func WaitDynamicReady(ctx context.Context) error {
	dynamicReadyMu.RLock()
	fn := dynamicReady
	dynamicReadyMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
