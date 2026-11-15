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

type scopedDynamicTestTool struct {
	*tool.Spec
	agents []string
}

func (scopedDynamicTestTool) IsDynamic() bool { return true }

func (t scopedDynamicTestTool) AvailableForAgent(agentName string) bool {
	if len(t.agents) == 0 {
		return true
	}
	for _, name := range t.agents {
		if name == agentName {
			return true
		}
	}
	return false
}

func TestAgentSyncsAllAgentScopedDynamicTools(t *testing.T) {
	const name = "test-scoped-all"
	spec := tool.NewSpec(name, "scoped dynamic tool", nil, func(context.Context, string) (string, error) {
		return "ok", nil
	})
	if err := tools.Register(scopedDynamicTestTool{Spec: spec}); err != nil {
		t.Fatal(err)
	}
	defer tools.Unregister(name)

	a := New("developer")
	if err := a.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Tools.Get(name); !ok {
		t.Fatal("all-agent dynamic tool was not added")
	}
}

func TestAgentScopesDynamicToolsByAgentName(t *testing.T) {
	const name = "test-scoped-orchestrator"
	spec := tool.NewSpec(name, "orchestrator-only tool", nil, func(context.Context, string) (string, error) {
		return "ok", nil
	})
	if err := tools.Register(scopedDynamicTestTool{Spec: spec, agents: []string{"orchestrator"}}); err != nil {
		t.Fatal(err)
	}
	defer tools.Unregister(name)

	orchestrator := New("orchestrator")
	if err := orchestrator.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := orchestrator.Tools.Get(name); !ok {
		t.Fatal("scoped tool was not added to the orchestrator")
	}

	developer := New("developer")
	if err := developer.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := developer.Tools.Get(name); ok {
		t.Fatal("scoped tool leaked into the developer agent")
	}
}

func TestAgentRemovesInheritedScopedDynamicTool(t *testing.T) {
	const name = "test-scoped-inherited"
	spec := tool.NewSpec(name, "orchestrator-only tool", nil, func(context.Context, string) (string, error) {
		return "ok", nil
	})
	if err := tools.Register(scopedDynamicTestTool{Spec: spec, agents: []string{"orchestrator"}}); err != nil {
		t.Fatal(err)
	}
	defer tools.Unregister(name)

	orchestrator := New("orchestrator")
	if err := orchestrator.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	developer := orchestrator.Clone("developer")
	if err := developer.resolveToolNames(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := developer.Tools.Get(name); ok {
		t.Fatal("inherited scoped tool was not removed from the developer clone")
	}
}
