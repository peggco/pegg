package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/core/config"
)

func TestManagerReadyWithoutServers(t *testing.T) {
	m := NewManager(newTestLogger())
	defer m.Close()

	if !m.Ready() {
		t.Fatal("manager should be ready with no servers")
	}
	if got := m.Pending(); got != 0 {
		t.Fatalf("pending = %d", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestManagerReloadRemovesServerTools(t *testing.T) {
	m := NewManager(newTestLogger())
	defer m.Close()

	const server = "test-reload"
	mt := NewTool(server, ToolSpec{Name: "echo", Description: "echo"}, nil)
	if err := tool.Default().Register(mt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tool.Default().Unregister(mt.Name()) })

	m.Reload(map[string]config.MCPServerConfig{
		server: {Type: "stdio", Command: "/nonexistent-pegg-test-binary"},
	})
	m.Reload(nil)

	if _, ok := tool.Default().Get(mt.Name()); ok {
		t.Fatal("tool should be removed when its server is removed")
	}
}
