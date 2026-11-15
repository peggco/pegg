package tui

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/tui/components"
)

func TestOnSubmitTransform(t *testing.T) {
	submitHook = hook.NewHook[string]()
	OnSubmit(func(s string) string { return "> " + s })
	if got := dispatchSubmit("hello"); got != "> hello" {
		t.Errorf("dispatchSubmit = %q, want > hello", got)
	}
	submitHook = hook.NewHook[string]()
}

func TestOnKeyConsume(t *testing.T) {
	keyHook = hook.NewHook[KeyEvent]()
	OnKey(func(ke KeyEvent) KeyEvent {
		ke.Consumed = true
		return ke
	})
	ke := dispatchKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if !ke.Consumed {
		t.Error("OnKey hook should be able to consume events")
	}
	keyHook = hook.NewHook[KeyEvent]()
}

func TestOnRegisterComponent(t *testing.T) {
	componentHook = hook.NewHook[[]components.Component]()
	type fake struct{ components.Component }
	OnRegisterComponent(func(c []components.Component) []components.Component {
		return append(c, fake{})
	})
	got := componentHook.Apply(nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 component after hook, got %d", len(got))
	}
}

func TestResolveGlobal(t *testing.T) {
	cases := []struct {
		key  tcell.Key
		rune rune
		mod  tcell.ModMask
		want keyAction
	}{
		{tcell.KeyCtrlC, 0, tcell.ModCtrl, ActionQuit},
		{tcell.KeyCtrlQ, 0, tcell.ModCtrl, ActionQuit},
		{tcell.KeyCtrlT, 0, tcell.ModCtrl, ActionThemeNext},
		{tcell.KeyCtrlG, 0, tcell.ModCtrl, ActionVoiceToggle},
		{tcell.KeyEnter, 0, 0, ActionNone},
		{tcell.KeyCtrlC, 0, tcell.ModCtrl | tcell.ModShift, ActionCopy},
		{tcell.KeyRune, 'c', tcell.ModCtrl | tcell.ModShift, ActionCopy},
		{tcell.KeyRune, 'x', tcell.ModCtrl | tcell.ModShift, ActionNone},
		{tcell.KeyTab, 0, 0, ActionModeNext},
		{tcell.KeyBacktab, 0, 0, ActionNone},
	}
	for _, c := range cases {
		ev := tcell.NewEventKey(c.key, c.rune, c.mod)
		if got := resolveGlobal(ev); got != c.want {
			t.Errorf("resolveGlobal(%v, %q) = %v, want %v", c.key, c.rune, got, c.want)
		}
	}
}

func withPluginHooks(t *testing.T) {
	t.Helper()
	hook.SetDefault(hook.NewBus())
	t.Cleanup(hook.ResetDefault)
}

func TestPluginSubmitHookRewritesMessage(t *testing.T) {
	withPluginHooks(t)
	hook.Default().Registry().AddFilterFor("test", HookSubmit, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["message"] = "hooked: " + payload["message"].(string)
		return payload, nil
	})
	if got := dispatchSubmit("hello"); got != "hooked: hello" {
		t.Fatalf("dispatchSubmit = %q", got)
	}
}

func TestPluginKeyHookConsumes(t *testing.T) {
	withPluginHooks(t)
	hook.Default().Registry().AddFilterFor("test", HookKey, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["consumed"] = true
		return payload, nil
	})
	ke := dispatchKey(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl))
	if !ke.Consumed {
		t.Fatal("plugin hook should consume the key")
	}
}

func TestPluginMessageAndNoticeHooks(t *testing.T) {
	withPluginHooks(t)
	hook.Default().Registry().AddFilterFor("test", HookMessage, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["content"] = "redacted"
		return payload, nil
	})
	hook.Default().Registry().AddFilterFor("test", HookNotice, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["body"] = "friendly error"
		return payload, nil
	})
	if got := applyMessageHook("a", "assistant", "", "secret"); got != "redacted" {
		t.Fatalf("message = %q", got)
	}
	if got := applyNoticeHook("error", "raw", "error"); got != "friendly error" {
		t.Fatalf("notice = %q", got)
	}
}

func TestPluginLifecycleActions(t *testing.T) {
	withPluginHooks(t)
	var ready, quit int
	hook.Default().Registry().AddActionFor("test", HookReady, 10, func(ctx context.Context, value any) (any, error) {
		ready++
		return nil, nil
	})
	hook.Default().Registry().AddActionFor("test", HookQuit, 10, func(ctx context.Context, value any) (any, error) {
		quit++
		return nil, nil
	})
	dispatchReady()
	dispatchQuit()
	if ready != 1 || quit != 1 {
		t.Fatalf("ready=%d quit=%d", ready, quit)
	}
}
