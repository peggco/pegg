package agent

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/agent/tools"
)

type dynamicTestTool struct {
	*tool.Spec
}

func (dynamicTestTool) IsDynamic() bool { return true }

func TestAgentSyncsDynamicTools(t *testing.T) {
	const name = "test-dynamic-mcp__echo"
	spec := tool.NewSpec(name, "dynamic test tool", nil, func(context.Context, string) (string, error) {
		return "ok", nil
	})
	if err := tools.Register(dynamicTestTool{spec}); err != nil {
		t.Fatal(err)
	}

	a := New("test")
	if err := a.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Tools.Get(name); !ok {
		t.Fatal("dynamic tool was not added to the agent")
	}

	tools.Unregister(name)
	if err := a.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Tools.Get(name); ok {
		t.Fatal("disconnected dynamic tool was not removed from the agent")
	}
}

func TestAgentDoesNotSyncStaticTools(t *testing.T) {
	const name = "test-static-tool"
	if err := tools.Register(tool.NewSpec(name, "static test tool", nil, func(context.Context, string) (string, error) {
		return "ok", nil
	})); err != nil {
		t.Fatal(err)
	}
	defer tools.Unregister(name)

	a := New("test")
	if err := a.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Tools.Get(name); ok {
		t.Fatal("static tool should only be added by name")
	}
}

func TestAgentWaitsForDynamicReady(t *testing.T) {
	called := false
	tools.SetDynamicReady(func(context.Context) error {
		called = true
		return nil
	})
	defer tools.SetDynamicReady(nil)

	a := New("test")
	if err := a.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("dynamic readiness hook was not called")
	}
}
