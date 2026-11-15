package tui

import (
	"context"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/styles"
)

type KeyEvent struct {
	Key      tcell.Key
	Rune     rune
	Mod      tcell.ModMask
	Consumed bool
}

var (
	submitHook    = hook.NewHook[string]()
	keyHook       = hook.NewHook[KeyEvent]()
	componentHook = hook.NewHook[[]components.Component]()
	readyHooks    = newCallbacks()
	quitHooks     = newCallbacks()
	busMu         sync.RWMutex
	appBus        event.Bus
)

type callbacks struct {
	mu  sync.Mutex
	fns []func()
}

func newCallbacks() *callbacks { return &callbacks{} }

func (c *callbacks) Add(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fn != nil {
		c.fns = append(c.fns, fn)
	}
}

func (c *callbacks) Run() {
	c.mu.Lock()
	fns := append([]func(){}, c.fns...)
	c.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func setBus(b event.Bus) {
	busMu.Lock()
	appBus = b
	busMu.Unlock()
}

func currentBus() event.Bus {
	busMu.RLock()
	defer busMu.RUnlock()
	return appBus
}

func OnSubmit(fn func(string) string) { submitHook.Add(fn) }

func OnKey(fn func(KeyEvent) KeyEvent) { keyHook.Add(fn) }

func OnRegisterComponent(fn func([]components.Component) []components.Component) {
	componentHook.Add(fn)
}

func OnReady(fn func()) { readyHooks.Add(fn) }

func OnQuit(fn func()) { quitHooks.Add(fn) }

const (
	HookSubmit  = "tui.submit"
	HookKey     = "tui.key"
	HookReady   = "tui.ready"
	HookQuit    = "tui.quit"
	HookTheme   = "tui.theme"
	HookNotice  = "tui.notice"
	HookMessage = "tui.message"

	HookStatusLeft       = "tui.status.left"
	HookStatusRight      = "tui.status.right"
	HookHeader           = "tui.header"
	HookFooter           = "tui.footer"
	HookChatItems        = "tui.chat.items"
	HookChatEmpty        = "tui.chat.empty"
	HookMessageDecorate  = "tui.message.decorate"
	HookPrompt           = "tui.prompt"
	HookMenuItems        = "tui.menu.items"
	HookKeyBindings      = "tui.keybindings"
	HookCommand          = "tui.command"
	HookSettingsSections = "tui.settings.sections"
	HookSettingsValues   = "tui.settings.values"
	HookSettingsSave     = "tui.settings.save"
)

type SubmitHook struct {
	Message string `json:"message"`
}

type KeyHook struct {
	Key      int   `json:"key"`
	Rune     int32 `json:"rune,omitempty"`
	Mod      int   `json:"mod,omitempty"`
	Consumed bool  `json:"consumed,omitempty"`
}

type ReadyHook struct {
	Workspace string `json:"workspace,omitempty"`
}

type ThemeHook struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

type NoticeHook struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body"`
	Level string `json:"level,omitempty"`
}

type MessageHook struct {
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content"`
}

func dispatchSubmit(msg string) string {
	msg = submitHook.Apply(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if hooked, errs := hook.ApplyFilter(ctx, HookSubmit, SubmitHook{Message: msg}); len(errs) == 0 && hooked.Message != "" {
		msg = hooked.Message
	}
	if bus := currentBus(); bus != nil {
		bus.Publish(TopicSubmit, SubmitEvent{Message: msg})
	}
	return msg
}

func dispatchKey(ev *tcell.EventKey) KeyEvent {
	ke := keyHook.Apply(KeyEvent{Key: ev.Key(), Rune: ev.Rune(), Mod: ev.Modifiers()})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if hooked, errs := hook.ApplyFilter(ctx, HookKey, KeyHook{
		Key:      int(ke.Key),
		Rune:     int32(ke.Rune),
		Mod:      int(ke.Mod),
		Consumed: ke.Consumed,
	}); len(errs) == 0 {
		ke.Consumed = hooked.Consumed
	}
	return ke
}

func dispatchReady() {
	readyHooks.Run()
	dispatchReadyHook("")
}

func dispatchQuit() {
	dispatchQuitHook()
	quitHooks.Run()
}

func dispatchReadyHook(workspace string) {
	hook.ApplyAction(context.Background(), HookReady, ReadyHook{Workspace: workspace})
}

func dispatchQuitHook() {
	hook.ApplyAction(context.Background(), HookQuit, struct{}{})
}

func applyThemeHook() {
	hooked, errs := hook.ApplyFilter(context.Background(), HookTheme, ThemeHook{Name: styles.Name()})
	if len(errs) == 0 && hooked.Name != "" && hooked.Name != styles.Name() {
		styles.Set(hooked.Name)
	}
}

func applyNoticeHook(title, body, level string) string {
	hooked, errs := hook.ApplyFilter(context.Background(), HookNotice, NoticeHook{Title: title, Body: body, Level: level})
	if len(errs) == 0 && hooked.Body != "" {
		return hooked.Body
	}
	return body
}

func applyMessageHook(agentID, agentName, role, content string) string {
	hooked, errs := hook.ApplyFilter(context.Background(), HookMessage, MessageHook{
		AgentID:   agentID,
		AgentName: agentName,
		Role:      role,
		Content:   content,
	})
	if len(errs) == 0 && hooked.Content != "" {
		return hooked.Content
	}
	return content
}
