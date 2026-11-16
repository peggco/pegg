package plugin

import (
	"context"
	"testing"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

type blockingClosePlugin struct {
	pkgplugin.Base
	name  string
	delay time.Duration
}

func (p *blockingClosePlugin) Name() string { return p.name }

func (p *blockingClosePlugin) Close() error {
	time.Sleep(p.delay)
	return nil
}

func TestCloseContextBoundsPluginShutdown(t *testing.T) {
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	for _, name := range []string{"one", "two", "three"} {
		mgr.plugins[name] = &PluginInstance{
			Name:   name,
			Plugin: &blockingClosePlugin{name: name, delay: 5 * time.Second},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	mgr.CloseContext(ctx)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("CloseContext took %s, want the deadline to bound it", elapsed)
	}
	if len(mgr.plugins) != 0 {
		t.Fatalf("plugins not cleared: %d", len(mgr.plugins))
	}
}
