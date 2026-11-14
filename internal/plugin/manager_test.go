package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

type hookPlugin struct {
	pkgplugin.Base
	name       string
	caps       []string
	hooks      []pkgplugin.HookSpec
	fetchCalls int
	execCalls  int
	exec       func(pkgplugin.HookRequest) (*pkgplugin.HookResult, error)
}

func (p *hookPlugin) Name() string           { return p.name }
func (p *hookPlugin) Capabilities() []string { return p.caps }
func (p *hookPlugin) Hooks() ([]pkgplugin.HookSpec, error) {
	p.fetchCalls++
	return p.hooks, nil
}

func (p *hookPlugin) ExecuteHook(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
	p.execCalls++
	if p.exec != nil {
		return p.exec(req)
	}
	return &pkgplugin.HookResult{}, nil
}

func newHookTestManager(t *testing.T, impls ...*hookPlugin) *Manager {
	t.Helper()
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	for _, impl := range impls {
		instance := &PluginInstance{
			Name:         impl.name,
			Plugin:       impl,
			Capabilities: impl.caps,
			Hooks:        impl.hooks,
		}
		mgr.plugins[impl.name] = instance
		mgr.registerHooks(instance)
	}
	return mgr
}

func appendTag(raw json.RawMessage, tag string) json.RawMessage {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		value = string(raw)
	}
	out, _ := json.Marshal(value + ":" + tag)
	return out
}

func TestApplyHookFilterOrderAndChaining(t *testing.T) {
	var order []string
	record := func(tag string) func(pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
			order = append(order, tag)
			if req.TimeoutMS != 1500 {
				t.Errorf("timeout for %s: %d", tag, req.TimeoutMS)
			}
			return &pkgplugin.HookResult{Value: appendTag(req.Value, tag), Handled: true}, nil
		}
	}
	spec := pkgplugin.HookSpec{Name: "test.filter", Kind: pkgplugin.HookFilter}
	a := &hookPlugin{name: "a", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 20}}, exec: record("a")}
	b := &hookPlugin{name: "b", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 5}}, exec: record("b")}
	c := &hookPlugin{name: "c", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 5}}, exec: record("c")}
	mgr := newHookTestManager(t, a, b, c)

	outcome, err := mgr.ApplyHook(context.Background(), spec.Name, pkgplugin.HookFilter, json.RawMessage(`"x"`), 1500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if string(outcome.Value) != `"x:b:c:a"` {
		t.Fatalf("value: %s", outcome.Value)
	}
	if strings.Join(order, ",") != "b,c,a" {
		t.Fatalf("order: %v", order)
	}
	if len(outcome.Errors) != 0 || len(outcome.Outputs) != 0 {
		t.Fatalf("outcome: %#v", outcome)
	}
}

func TestApplyHookErrorIsolation(t *testing.T) {
	spec := pkgplugin.HookSpec{Name: "test.filter", Kind: pkgplugin.HookFilter}
	var order []string
	a := &hookPlugin{name: "a", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 30}}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		order = append(order, "a")
		return &pkgplugin.HookResult{Value: appendTag(req.Value, "a"), Handled: true}, nil
	}}
	b := &hookPlugin{name: "b", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 5}}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		order = append(order, "b")
		return nil, errors.New("boom")
	}}
	c := &hookPlugin{name: "c", hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: 10}}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		order = append(order, "c")
		return &pkgplugin.HookResult{Value: appendTag(req.Value, "c"), Handled: true}, nil
	}}
	mgr := newHookTestManager(t, a, b, c)

	outcome, err := mgr.ApplyHook(context.Background(), spec.Name, pkgplugin.HookFilter, json.RawMessage(`"x"`), 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "b,c,a" {
		t.Fatalf("order: %v", order)
	}
	if string(outcome.Value) != `"x:c:a"` {
		t.Fatalf("value: %s", outcome.Value)
	}
	if len(outcome.Errors) != 1 || !strings.Contains(outcome.Errors[0], "b") || !strings.Contains(outcome.Errors[0], "boom") {
		t.Fatalf("errors: %#v", outcome.Errors)
	}
}

func TestApplyHookActions(t *testing.T) {
	spec := pkgplugin.HookSpec{Name: "test.action", Kind: pkgplugin.HookAction}
	var inputs []string
	makePlugin := func(name string, priority int, output string, fail bool) *hookPlugin {
		return &hookPlugin{name: name, hooks: []pkgplugin.HookSpec{{Name: spec.Name, Kind: spec.Kind, Priority: priority}}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
			inputs = append(inputs, string(req.Value))
			if fail {
				return nil, errors.New("failed")
			}
			return &pkgplugin.HookResult{Value: json.RawMessage(output)}, nil
		}}
	}
	a := makePlugin("a", 10, `"one"`, false)
	b := makePlugin("b", 5, `"two"`, false)
	c := makePlugin("c", 20, `"three"`, true)
	mgr := newHookTestManager(t, a, b, c)

	outcome, err := mgr.ApplyHook(context.Background(), spec.Name, pkgplugin.HookAction, json.RawMessage(`{"payload":true}`), 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(outcome.Value) != `{"payload":true}` {
		t.Fatalf("action value: %s", outcome.Value)
	}
	if len(outcome.Outputs) != 2 || string(outcome.Outputs[0]) != `"two"` || string(outcome.Outputs[1]) != `"one"` {
		t.Fatalf("outputs: %#v", outcome.Outputs)
	}
	if len(outcome.Errors) != 1 || !strings.Contains(outcome.Errors[0], "c") {
		t.Fatalf("errors: %#v", outcome.Errors)
	}
	for _, input := range inputs {
		if input != `{"payload":true}` {
			t.Fatalf("action input: %s", input)
		}
	}
}

func TestApplyHookMissingHandler(t *testing.T) {
	mgr := newHookTestManager(t)
	if _, err := mgr.ApplyHook(context.Background(), "missing.hook", pkgplugin.HookFilter, nil, 0); err == nil {
		t.Fatal("expected no-handler error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mgr.ApplyHook(ctx, "missing.hook", pkgplugin.HookFilter, nil, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestFetchHooksRequiresExplicitCapability(t *testing.T) {
	silent := &hookPlugin{
		name: "silent",
		hooks: []pkgplugin.HookSpec{
			{Name: "test.filter", Kind: pkgplugin.HookFilter, Priority: 5},
		},
	}
	mgr := newHookTestManager(t)
	instance := &PluginInstance{Name: silent.name, Plugin: silent}
	mgr.fetchCapabilities(instance)
	if silent.fetchCalls != 0 {
		t.Fatalf("nil capabilities fetched hooks %d times", silent.fetchCalls)
	}
	if instance.Hooks != nil {
		t.Fatalf("nil capabilities stored hooks: %#v", instance.Hooks)
	}

	declared := &hookPlugin{
		name: "declared",
		caps: []string{string(pkgplugin.CapabilityHooks)},
		hooks: []pkgplugin.HookSpec{
			{Name: " ", Kind: pkgplugin.HookAction},
			{Name: " test.filter ", Kind: "", Priority: 0, Description: "normalized"},
			{Name: "test.action", Kind: pkgplugin.HookAction, Priority: 3},
			{Name: "test.unknown", Kind: "weird", Priority: 7},
		},
	}
	declaredInstance := &PluginInstance{Name: declared.name, Plugin: declared, Capabilities: declared.caps}
	mgr.fetchCapabilities(declaredInstance)
	if declared.fetchCalls != 1 {
		t.Fatalf("fetch calls: %d", declared.fetchCalls)
	}
	if len(declaredInstance.Hooks) != 3 {
		t.Fatalf("normalized hooks: %#v", declaredInstance.Hooks)
	}
	if got := declaredInstance.Hooks[0]; got.Name != "test.filter" || got.Kind != pkgplugin.HookFilter || got.Priority != 10 {
		t.Fatalf("normalized filter: %#v", got)
	}
	if got := declaredInstance.Hooks[1]; got.Name != "test.action" || got.Kind != pkgplugin.HookAction || got.Priority != 3 {
		t.Fatalf("normalized action: %#v", got)
	}
	if got := declaredInstance.Hooks[2]; got.Kind != pkgplugin.HookFilter || got.Priority != 7 {
		t.Fatalf("normalized unknown kind: %#v", got)
	}

	mgr.mu.Lock()
	mgr.plugins[declared.name] = declaredInstance
	mgr.mu.Unlock()
	specs := mgr.HookSpecs()
	if len(specs) != 3 {
		t.Fatalf("specs: %#v", specs)
	}
	for i := 1; i < len(specs); i++ {
		if specs[i-1].Name > specs[i].Name {
			t.Fatalf("specs not sorted: %#v", specs)
		}
	}
}

func TestDispatchHookSkipsOrigin(t *testing.T) {
	spec := pkgplugin.HookSpec{Name: "kanban.providers", Kind: pkgplugin.HookFilter}
	var ran []string
	makePlugin := func(name string) *hookPlugin {
		return &hookPlugin{name: name, hooks: []pkgplugin.HookSpec{spec}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
			ran = append(ran, name)
			return &pkgplugin.HookResult{Value: appendTag(req.Value, name), Handled: true}, nil
		}}
	}
	mgr := newHookTestManager(t, makePlugin("kanban"), makePlugin("github"))

	res, err := mgr.dispatchHook(pkgplugin.HookRequest{
		Name:      spec.Name,
		Kind:      pkgplugin.HookFilter,
		Value:     json.RawMessage(`"v"`),
		Origin:    "kanban",
		TimeoutMS: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ran, ",") != "github" {
		t.Fatalf("ran: %v", ran)
	}
	if string(res.Value) != `"v:github"` {
		t.Fatalf("value: %s", res.Value)
	}
	if !res.Handled {
		t.Fatal("expected handled result")
	}
}

func TestDispatchHookSelfOnlyIsNoHandler(t *testing.T) {
	spec := pkgplugin.HookSpec{Name: "kanban.providers", Kind: pkgplugin.HookFilter}
	only := &hookPlugin{name: "kanban", hooks: []pkgplugin.HookSpec{spec}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return &pkgplugin.HookResult{Value: req.Value, Handled: true}, nil
	}}
	mgr := newHookTestManager(t, only)

	_, err := mgr.dispatchHook(pkgplugin.HookRequest{
		Name:   spec.Name,
		Kind:   pkgplugin.HookFilter,
		Origin: "kanban",
	})
	if err == nil || !strings.Contains(err.Error(), "no handler registered") {
		t.Fatalf("error: %v", err)
	}
}

func TestDispatchHookActionsReturnOutputs(t *testing.T) {
	spec := pkgplugin.HookSpec{Name: "kanban.card.succeeded", Kind: pkgplugin.HookAction}
	handler := &hookPlugin{name: "jira", hooks: []pkgplugin.HookSpec{spec}, exec: func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return &pkgplugin.HookResult{Value: json.RawMessage(`"done"`)}, nil
	}}
	mgr := newHookTestManager(t, handler)

	res, err := mgr.dispatchHook(pkgplugin.HookRequest{
		Name:      spec.Name,
		Kind:      pkgplugin.HookAction,
		Value:     json.RawMessage(`{"card":"c1"}`),
		Origin:    "kanban",
		TimeoutMS: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outputs) != 1 || string(res.Outputs[0]) != `"done"` {
		t.Fatalf("outputs: %#v", res.Outputs)
	}
}

func TestDispatchHookRecursionLimit(t *testing.T) {
	mgr := newHookTestManager(t)
	key := "alpha\x00loop.hook"
	for i := 0; i < maxHookDepth; i++ {
		if !mgr.enterHook(key) {
			t.Fatalf("enter %d rejected", i)
		}
	}
	_, err := mgr.dispatchHook(pkgplugin.HookRequest{Name: "loop.hook", Kind: pkgplugin.HookFilter, Origin: "alpha"})
	if err == nil || !strings.Contains(err.Error(), "recursion limit") {
		t.Fatalf("error: %v", err)
	}
	for i := 0; i < maxHookDepth; i++ {
		mgr.leaveHook(key)
	}
	if !mgr.enterHook(key) {
		t.Fatal("depth was not released")
	}
	mgr.leaveHook(key)
}

func TestLockHookExecHonorsContext(t *testing.T) {
	instance := &PluginInstance{}
	instance.execMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lockHookExec(ctx, instance) {
		t.Fatal("acquired a held mutex")
	}
	instance.execMu.Unlock()
	if !lockHookExec(context.Background(), instance) {
		t.Fatal("did not acquire a free mutex")
	}
	instance.execMu.Unlock()
}
