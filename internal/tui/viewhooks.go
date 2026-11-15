package tui

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/tui/components"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

const (
	viewHookTimeout  = 150 * time.Millisecond
	viewHookInterval = 500 * time.Millisecond
)

type viewState struct {
	statusLeft   []pkgplugin.ViewSpan
	statusRight  []pkgplugin.ViewSpan
	replaceLeft  bool
	replaceRight bool
	header       []pkgplugin.ViewLine
	footer       pkgplugin.FooterHook
	chatItems    []pkgplugin.ViewFragment
	emptyItems   []pkgplugin.ViewFragment
	prompt       pkgplugin.PromptHook
	bindings     []pkgplugin.ViewKeyBinding
}

type viewHooks struct {
	app        *App
	mu         sync.RWMutex
	state      viewState
	refreshing atomic.Bool
	last       atomic.Int64
}

func newViewHooks(app *App) *viewHooks {
	return &viewHooks{app: app}
}

func (v *viewHooks) snapshot() viewState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

func (v *viewHooks) refresh(ctx context.Context) {
	if !v.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer v.refreshing.Store(false)
	v.last.Store(time.Now().UnixNano())

	next := v.snapshot()
	statusCtx := v.app.statusHook()
	viewCtx := v.app.viewContext()

	apply := func(fn func(context.Context)) {
		callCtx, cancel := context.WithTimeout(ctx, viewHookTimeout)
		defer cancel()
		fn(callCtx)
	}

	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookStatusLeft, statusCtx); len(errs) == 0 {
			next.statusLeft, next.replaceLeft = out.Spans, out.Replace
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookStatusRight, statusCtx); len(errs) == 0 {
			next.statusRight, next.replaceRight = out.Spans, out.Replace
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookHeader, pkgplugin.HeaderHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.header = headerLines(out)
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookFooter, pkgplugin.FooterHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.footer = out
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookChatItems, pkgplugin.ChatItemsHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.chatItems = out.Items
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookChatEmpty, pkgplugin.ChatItemsHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.emptyItems = out.Items
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookPrompt, pkgplugin.PromptHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.prompt = out
		}
	})
	apply(func(callCtx context.Context) {
		if out, errs := hook.ApplyFilter(callCtx, HookKeyBindings, pkgplugin.KeyBindingsHook{ViewContext: viewCtx}); len(errs) == 0 {
			next.bindings = out.Bindings
		}
	})

	v.mu.Lock()
	v.state = next
	v.mu.Unlock()

	v.app.postUI(func() { v.apply(next) })
}

func (v *viewHooks) apply(state viewState) {
	app := v.app
	if app.home != nil {
		footer := state.footer
		if footer.Hint == "" && len(state.bindings) > 0 {
			footer.Hint = bindingHint(state.bindings)
		}
		app.home.SetViewChrome(state.header, footer)
		app.home.SetEmptyFragments(state.emptyItems)
		app.home.SetPromptPlaceholder(state.prompt.Placeholder)
		app.home.StatusBar().SetViewSpans(state.statusLeft, state.statusRight, state.replaceLeft, state.replaceRight)
	}
	app.syncPluginChatItems(state.chatItems)
	app.requestRedraw()
}

func bindingHint(bindings []pkgplugin.ViewKeyBinding) string {
	labels := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		label := binding.Label
		if label == "" {
			label = binding.Key
		}
		if label == "" {
			continue
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, " · ")
}

func headerLines(hookResult pkgplugin.HeaderHook) []pkgplugin.ViewLine {
	lines := make([]pkgplugin.ViewLine, 0, len(hookResult.Lines)+1)
	if hookResult.Title != "" {
		lines = append(lines, pkgplugin.ViewLine{Spans: []pkgplugin.ViewSpan{{Text: hookResult.Title, Style: "accent"}}})
	}
	return append(lines, hookResult.Lines...)
}

func (v *viewHooks) bindings() []pkgplugin.ViewKeyBinding {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state.bindings
}

func (a *App) refreshViewHooks() {
	if a.viewHooks == nil {
		return
	}
	go a.viewHooks.refresh(context.Background())
}

func (a *App) statusHook() pkgplugin.StatusHook {
	status := pkgplugin.StatusContext{
		Mode:        string(a.mode),
		Running:     a.running,
		Cost:        a.usage.Cost,
		TotalTokens: a.usage.TotalTokens,
	}
	if a.model.provider != "" {
		status.Provider = a.model.provider
	}
	if a.model.model.ID != "" {
		status.Model = a.model.model.ID
	}
	if a.home != nil && a.chatMu.TryLock() {
		snapshot := a.home.StatusBar().Snapshot()
		a.chatMu.Unlock()
		status.Mode = snapshot.Mode
		status.Running = snapshot.Running
		status.Provider = snapshot.Provider
		status.Model = snapshot.ModelName
		status.ReasoningEffort = snapshot.ReasoningEffort
		status.MaxTokens = snapshot.MaxInputTokens
		status.Cost = snapshot.Usage.Cost
		status.TotalTokens = snapshot.Usage.TotalTokens
		status.Current = statusSpans(snapshot)
	}
	return pkgplugin.StatusHook{StatusContext: status}
}

func statusSpans(snapshot components.StatusSnapshot) []pkgplugin.ViewSpan {
	var spans []pkgplugin.ViewSpan
	if snapshot.Mode != "" {
		spans = append(spans, pkgplugin.ViewSpan{Text: snapshot.Mode, Style: "accent"})
	}
	if snapshot.ModelName != "" {
		spans = append(spans, pkgplugin.ViewSpan{Text: snapshot.ModelName})
	}
	if snapshot.Provider != "" {
		spans = append(spans, pkgplugin.ViewSpan{Text: snapshot.Provider, Style: "hint"})
	}
	return spans
}

func (a *App) viewContext() pkgplugin.ViewContext {
	view := pkgplugin.ViewContext{
		Mode:    string(a.mode),
		Running: a.running,
	}
	if a.session != nil {
		view.SessionID = a.session.info.ID
	}
	if a.agent != nil {
		view.AgentID = a.agent.ID
		view.AgentName = a.agent.Name
	}
	if wd, err := os.Getwd(); err == nil {
		view.Workspace = wd
	}
	if a.chat != nil && a.chatMu.TryLock() {
		view.Messages = len(a.chat.Items())
		a.chatMu.Unlock()
	}
	return view
}
