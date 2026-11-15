package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/plugin"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type fakeHookManager struct {
	specs   []plugin.PluginHookSpec
	outcome plugin.HookOutcome
	err     error
	closed  bool
	calls   int
	name    string
	kind    pkgplugin.HookKind
	value   json.RawMessage
	timeout time.Duration
}

func (f *fakeHookManager) HookSpecs() []plugin.PluginHookSpec { return f.specs }

func (f *fakeHookManager) ApplyHook(ctx context.Context, name string, kind pkgplugin.HookKind, value json.RawMessage, timeout time.Duration) (plugin.HookOutcome, error) {
	f.calls++
	f.name = name
	f.kind = kind
	f.value = value
	f.timeout = timeout
	return f.outcome, f.err
}

func (f *fakeHookManager) Close() { f.closed = true }

func newHooksCLI(t *testing.T, fake *fakeHookManager) (*CLI, *bytes.Buffer) {
	t.Helper()
	c, _ := newTestCLI(t)
	c.hooksBackend = func() (hookManager, error) { return fake, nil }
	var out bytes.Buffer
	c.root.SetOut(&out)
	c.root.SetErr(&out)
	return c, &out
}

func TestHooksList(t *testing.T) {
	fake := &fakeHookManager{specs: []plugin.PluginHookSpec{
		{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter, Priority: 10, Description: "providers"},
		{Plugin: "beta", Name: "kanban.card.succeeded", Kind: pkgplugin.HookAction, Priority: 20, Description: "notify"},
	}}
	c, out := newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "list"}); err != nil {
		t.Fatalf("list: %v\n%s", err, out.String())
	}
	for _, want := range []string{"alpha", "kanban.providers", "filter", "10", "beta", "kanban.card.succeeded", "action", "20"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output:\n%s", want, out.String())
		}
	}
	if !fake.closed {
		t.Fatal("manager not closed")
	}
}

func TestHooksListJSON(t *testing.T) {
	fake := &fakeHookManager{specs: []plugin.PluginHookSpec{
		{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter, Priority: 10, Description: "providers"},
	}}
	c, out := newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "list", "--json"}); err != nil {
		t.Fatalf("list: %v\n%s", err, out.String())
	}
	var entries []hookListEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if len(entries) != 1 || entries[0].Plugin != "alpha" || entries[0].Name != "kanban.providers" || entries[0].Kind != "filter" || entries[0].Priority != 10 {
		t.Fatalf("entries: %#v", entries)
	}
}

func TestHooksListEmpty(t *testing.T) {
	c, out := newHooksCLI(t, &fakeHookManager{})
	if err := c.Execute([]string{"hooks", "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No hooks registered") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestHooksApplyJSON(t *testing.T) {
	fake := &fakeHookManager{
		specs: []plugin.PluginHookSpec{
			{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter, Priority: 10},
		},
		outcome: plugin.HookOutcome{
			Value:   json.RawMessage(`"result"`),
			Outputs: []json.RawMessage{json.RawMessage(`"out"`)},
			Errors:  []string{"handler failed"},
		},
	}
	c, out := newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "filter", "--value", `{"x":1}`, "--json"}); err != nil {
		t.Fatalf("apply: %v\n%s", err, out.String())
	}
	if fake.calls != 1 || fake.name != "kanban.providers" || fake.kind != pkgplugin.HookFilter {
		t.Fatalf("call: %#v", fake)
	}
	if string(fake.value) != `{"x":1}` {
		t.Fatalf("value: %s", fake.value)
	}
	if fake.timeout != 30*time.Second {
		t.Fatalf("timeout: %s", fake.timeout)
	}
	var got struct {
		Value   json.RawMessage   `json:"value"`
		Outputs []json.RawMessage `json:"outputs"`
		Errors  []string          `json:"errors"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if string(got.Value) != `"result"` || len(got.Outputs) != 1 || len(got.Errors) != 1 || got.Errors[0] != "handler failed" {
		t.Fatalf("output: %#v", got)
	}
	if !fake.closed {
		t.Fatal("manager not closed")
	}
}

func TestHooksApplyInfersKindAndReadsStdin(t *testing.T) {
	fake := &fakeHookManager{
		specs: []plugin.PluginHookSpec{
			{Plugin: "alpha", Name: "kanban.issues.fetch", Kind: pkgplugin.HookFilter, Priority: 10},
		},
		outcome: plugin.HookOutcome{Value: json.RawMessage(`"payload"`)},
	}
	c, out := newHooksCLI(t, fake)
	c.root.SetIn(strings.NewReader(`"payload"`))
	if err := c.Execute([]string{"hooks", "apply", "kanban.issues.fetch", "--value", "-"}); err != nil {
		t.Fatalf("apply: %v\n%s", err, out.String())
	}
	if fake.kind != pkgplugin.HookFilter {
		t.Fatalf("kind: %s", fake.kind)
	}
	if string(fake.value) != `"payload"` {
		t.Fatalf("value: %s", fake.value)
	}
}

func TestHooksApplyReadsPipedStdinWhenValueOmitted(t *testing.T) {
	fake := &fakeHookManager{
		specs: []plugin.PluginHookSpec{
			{Plugin: "alpha", Name: "kanban.issues.fetch", Kind: pkgplugin.HookFilter, Priority: 10},
		},
	}
	c, _ := newHooksCLI(t, fake)
	c.root.SetIn(strings.NewReader(`{"limit":5}`))
	if err := c.Execute([]string{"hooks", "apply", "kanban.issues.fetch"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if string(fake.value) != `{"limit":5}` {
		t.Fatalf("value: %s", fake.value)
	}
}

func TestHooksApplyRequiresKindWhenAmbiguous(t *testing.T) {
	fake := &fakeHookManager{specs: []plugin.PluginHookSpec{
		{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter},
		{Plugin: "beta", Name: "kanban.providers", Kind: pkgplugin.HookAction},
	}}
	c, _ := newHooksCLI(t, fake)
	err := c.Execute([]string{"hooks", "apply", "kanban.providers"})
	if err == nil || !strings.Contains(err.Error(), "specify --kind") {
		t.Fatalf("error: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("handler called %d times", fake.calls)
	}
}

func TestHooksApplyNoHandler(t *testing.T) {
	fake := &fakeHookManager{}
	c, _ := newHooksCLI(t, fake)
	err := c.Execute([]string{"hooks", "apply", "missing.hook"})
	if err == nil || !strings.Contains(err.Error(), "no handler") {
		t.Fatalf("error: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("handler called %d times", fake.calls)
	}
}

func TestHooksApplyReturnsManagerError(t *testing.T) {
	fake := &fakeHookManager{
		specs: []plugin.PluginHookSpec{
			{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter},
		},
		err: errors.New("transport failure"),
	}
	c, _ := newHooksCLI(t, fake)
	err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "filter", "--value", "null"})
	if err == nil || !strings.Contains(err.Error(), "transport failure") {
		t.Fatalf("error: %v", err)
	}
}

func TestHooksApplyKeepsHandlerErrorsInJSON(t *testing.T) {
	fake := &fakeHookManager{
		specs: []plugin.PluginHookSpec{
			{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter},
		},
		outcome: plugin.HookOutcome{Errors: []string{"alpha: boom"}},
	}
	c, out := newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "filter", "--value", "null", "--json"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var got struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if len(got.Errors) != 1 || got.Errors[0] != "alpha: boom" {
		t.Fatalf("errors: %#v", got.Errors)
	}
}

func TestHooksApplyRejectsInvalidKindAndValue(t *testing.T) {
	fake := &fakeHookManager{specs: []plugin.PluginHookSpec{
		{Plugin: "alpha", Name: "kanban.providers", Kind: pkgplugin.HookFilter},
	}}
	c, _ := newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "other"}); err == nil || !strings.Contains(err.Error(), "invalid --kind") {
		t.Fatalf("kind error: %v", err)
	}

	c, _ = newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "filter", "--value", "not json"}); err == nil || !strings.Contains(err.Error(), "invalid hook value") {
		t.Fatalf("value error: %v", err)
	}

	c, _ = newHooksCLI(t, fake)
	if err := c.Execute([]string{"hooks", "apply", "kanban.providers", "--kind", "filter", "--timeout", "0s"}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("timeout error: %v", err)
	}
}
