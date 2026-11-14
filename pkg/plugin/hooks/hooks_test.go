package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func withDispatcher(t *testing.T, fn pkgplugin.HookDispatchFunc) {
	t.Helper()
	previous := pkgplugin.HostDispatcher()
	pkgplugin.SetHostDispatcher(fn)
	t.Cleanup(func() { pkgplugin.SetHostDispatcher(previous) })
}

func TestApplyDispatchesThroughHost(t *testing.T) {
	var captured pkgplugin.HookRequest
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		captured = req
		return &pkgplugin.HookResult{
			Value:   json.RawMessage(`"filtered"`),
			Handled: true,
			Outputs: []json.RawMessage{json.RawMessage(`"o1"`)},
			Errors:  []string{"e1"},
		}, nil
	})

	client := &Client{Timeout: 5 * time.Second}
	outcome, err := client.Apply(context.Background(), "my.hook", HookFilter, map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if string(outcome.Value) != `"filtered"` || len(outcome.Outputs) != 1 || len(outcome.Errors) != 1 {
		t.Fatalf("outcome: %#v", outcome)
	}
	if captured.Name != "my.hook" || captured.Kind != HookFilter {
		t.Fatalf("request: %#v", captured)
	}
	if string(captured.Value) != `{"a":"b"}` {
		t.Fatalf("payload: %s", captured.Value)
	}
	if captured.TimeoutMS != 5000 {
		t.Fatalf("timeout: %d", captured.TimeoutMS)
	}
}

func TestApplyWithoutHostDispatcher(t *testing.T) {
	withDispatcher(t, nil)
	client := &Client{}
	_, err := client.Apply(context.Background(), "my.hook", HookFilter, nil)
	if !errors.Is(err, ErrNoHostDispatcher) {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyNoHandler(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return nil, errors.New(`hook "my.hook" (filter): no handler registered`)
	})
	client := &Client{}
	_, err := client.Apply(context.Background(), "my.hook", HookFilter, nil)
	if !errors.Is(err, ErrNoHandler) {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyHostError(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return nil, errors.New("boom")
	})
	client := &Client{}
	_, err := client.Apply(context.Background(), "my.hook", HookFilter, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "my.hook") {
		t.Fatalf("error: %v", err)
	}
}

func TestFilterIntoDecodesValue(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		if req.Name != "my.filter" || req.Kind != HookFilter {
			t.Fatalf("request: %#v", req)
		}
		return &pkgplugin.HookResult{Value: json.RawMessage(`"changed"`), Handled: true}, nil
	})
	client := &Client{}
	var out string
	outcome, err := client.FilterInto(context.Background(), "my.filter", "input", &out)
	if err != nil {
		t.Fatal(err)
	}
	if out != "changed" || string(outcome.Value) != `"changed"` {
		t.Fatalf("out: %q outcome: %#v", out, outcome)
	}
}

func TestFilterIntoDecodeError(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return &pkgplugin.HookResult{Value: json.RawMessage(`{"not":"a string"}`), Handled: true}, nil
	})
	client := &Client{}
	var out string
	if _, err := client.FilterInto(context.Background(), "my.filter", "input", &out); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestActionReturnsOutputs(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		if req.Kind != HookAction || string(req.Value) != `{"ok":true}` {
			t.Fatalf("request: %#v", req)
		}
		return &pkgplugin.HookResult{
			Outputs: []json.RawMessage{json.RawMessage(`"one"`), json.RawMessage(`"two"`)},
		}, nil
	})
	client := &Client{}
	outcome, err := client.Action(context.Background(), "my.action", map[string]bool{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Outputs) != 2 || string(outcome.Outputs[0]) != `"one"` || string(outcome.Outputs[1]) != `"two"` {
		t.Fatalf("outputs: %#v", outcome.Outputs)
	}
}

func TestApplyRespectsContext(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		<-release
		return &pkgplugin.HookResult{}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := &Client{}
	_, err := client.Apply(ctx, "slow.hook", HookFilter, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyValidation(t *testing.T) {
	withDispatcher(t, func(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
		return &pkgplugin.HookResult{}, nil
	})
	client := &Client{}
	if _, err := client.Apply(context.Background(), " ", HookFilter, nil); err == nil {
		t.Fatal("expected missing name error")
	}
	if _, err := client.Apply(context.Background(), "h", pkgplugin.HookKind("bogus"), nil); err == nil {
		t.Fatal("expected unknown kind error")
	}
}

func TestCloseIsNoop(t *testing.T) {
	if err := (&Client{}).Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
