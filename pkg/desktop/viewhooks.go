//go:build gui

package main

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corehook "github.com/peggco/pegg/internal/core/hook"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

const (
	desktopViewTimeout     = 150 * time.Millisecond
	desktopViewMinInterval = 250 * time.Millisecond
	desktopViewTick        = time.Second
	desktopViewOwner       = "@view"
)

type DesktopViewState struct {
	Header        []pkgplugin.ViewLine       `json:"header,omitempty"`
	Footer        []pkgplugin.ViewLine       `json:"footer,omitempty"`
	StatusLeft    []pkgplugin.ViewSpan       `json:"statusLeft,omitempty"`
	StatusRight   []pkgplugin.ViewSpan       `json:"statusRight,omitempty"`
	ReplaceStatus bool                       `json:"replaceStatus,omitempty"`
	ChatItems     []pkgplugin.ViewFragment   `json:"chatItems,omitempty"`
	EmptyItems    []pkgplugin.ViewFragment   `json:"emptyItems,omitempty"`
	Placeholder   string                     `json:"placeholder,omitempty"`
	MenuItems     []pkgplugin.ViewMenuItem   `json:"menuItems,omitempty"`
	KeyBindings   []pkgplugin.ViewKeyBinding `json:"keyBindings,omitempty"`
	SettingsPages []PluginSettingPage        `json:"settingsPages,omitempty"`
}

type desktopViewHooks struct {
	app *App

	mu    sync.RWMutex
	raw   pkgplugin.DesktopView
	state DesktopViewState

	refreshing atomic.Bool
	last       atomic.Int64
}

func newDesktopViewHooks(app *App) *desktopViewHooks {
	return &desktopViewHooks{app: app}
}

func (v *desktopViewHooks) snapshot() DesktopViewState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

func (v *desktopViewHooks) refresh(ctx context.Context) {
	if !v.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer v.refreshing.Store(false)
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UnixNano()
	if previous := v.last.Load(); previous != 0 && now-previous < int64(desktopViewMinInterval) {
		return
	}
	v.last.Store(now)

	if !corehook.HasFilter(HookView) {
		v.store(pkgplugin.DesktopView{}, DesktopViewState{})
		return
	}

	v.mu.RLock()
	previous := v.raw
	v.mu.RUnlock()

	callCtx, cancel := context.WithTimeout(ctx, desktopViewTimeout)
	out, errs := corehook.ApplyFilter(callCtx, HookView, pkgplugin.DesktopViewHook{
		DesktopViewContext: v.app.desktopViewContext(),
		View:               previous,
	})
	cancel()
	if len(errs) > 0 {
		return
	}
	v.store(out.View, desktopViewState(out.View))
}

func (v *desktopViewHooks) store(raw pkgplugin.DesktopView, state DesktopViewState) {
	v.mu.Lock()
	v.raw = raw
	v.state = state
	v.mu.Unlock()
	v.app.emit("desktop:view", state)
}

func (v *desktopViewHooks) saveSettings(tab string, values map[string]string) {
	callCtx, cancel := context.WithTimeout(context.Background(), desktopViewTimeout)
	_, errs := corehook.ApplyAction(callCtx, HookViewSettings, pkgplugin.SettingsSaveHook{
		Tab:    tab,
		Values: values,
	})
	cancel()
	if len(errs) == 0 {
		v.app.refreshDesktopView()
	}
}

func desktopViewState(view pkgplugin.DesktopView) DesktopViewState {
	return DesktopViewState{
		Header:        view.Header,
		Footer:        view.Footer,
		StatusLeft:    view.StatusLeft,
		StatusRight:   view.StatusRight,
		ReplaceStatus: view.ReplaceStatus,
		ChatItems:     view.ChatItems,
		EmptyItems:    view.EmptyItems,
		Placeholder:   view.Placeholder,
		MenuItems:     view.MenuItems,
		KeyBindings:   view.KeyBindings,
		SettingsPages: viewSettingsPages(view.Settings),
	}
}

func viewSettingsPages(sections []pkgplugin.ViewSection) []PluginSettingPage {
	groups := make(map[string][]pkgplugin.ViewSection)
	order := make([]string, 0, len(sections))
	for _, section := range sections {
		key := strings.ToLower(strings.TrimSpace(section.Tab))
		if key == "" {
			key = "extensions"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], section)
	}

	pages := make([]PluginSettingPage, 0, len(order))
	for _, key := range order {
		page := PluginSettingPage{
			ID:         "view:" + key,
			Title:      viewSettingsTitle(key),
			Owner:      desktopViewOwner,
			SaveAction: "save",
		}
		for _, section := range groups[key] {
			page.Sections = append(page.Sections, PluginSettingSection{
				Title: section.Title,
				Kind:  "plugin",
				Note:  section.Description,
			})
			for _, field := range section.Fields {
				page.Settings = append(page.Settings, PluginSettingField{
					ID:          field.ID,
					Label:       field.Label,
					Placeholder: field.Placeholder,
					Value:       field.Value,
					Secret:      field.Secret,
					Required:    field.Required,
					Options:     field.Options,
				})
			}
		}
		pages = append(pages, page)
	}
	return pages
}

func viewSettingsTitle(key string) string {
	switch key {
	case "extensions":
		return "Extensions"
	case "general":
		return "General"
	case "session":
		return "Session"
	case "mcp":
		return "MCP"
	case "skills":
		return "Skills"
	case "rules":
		return "Rules"
	case "plugins":
		return "Plugins"
	case "permissions":
		return "Permissions"
	case "memory":
		return "Memory"
	case "system":
		return "System"
	default:
		if key == "" {
			return "Extensions"
		}
		return strings.ToUpper(key[:1]) + key[1:]
	}
}

func (a *App) desktopViewContext() pkgplugin.DesktopViewContext {
	viewCtx := pkgplugin.DesktopViewContext{
		Platform:  runtime.GOOS,
		Workspace: a.currentWorkspace(),
	}
	if a.prefs != nil {
		viewCtx.Theme = a.prefs.Get().Theme
	}
	a.sessionsMu.Lock()
	viewCtx.Running = len(a.pending) > 0
	a.sessionsMu.Unlock()
	if session := a.CurrentSession(); session != nil {
		viewCtx.SessionID = session.PeggID
		if viewCtx.SessionID == "" {
			viewCtx.SessionID = session.ACPID
		}
		viewCtx.Mode = session.Mode
		viewCtx.AgentName = session.Title
	}
	return viewCtx
}

func (a *App) refreshDesktopView() {
	if a.viewHooks == nil {
		return
	}
	go a.viewHooks.refresh(context.Background())
}

func (a *App) desktopViewLoop(ctx context.Context) {
	ticker := time.NewTicker(desktopViewTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if corehook.HasFilter(HookView) {
				a.refreshDesktopView()
			}
		}
	}
}

func (a *App) ViewState() DesktopViewState {
	if a.viewHooks == nil {
		return DesktopViewState{}
	}
	return a.viewHooks.snapshot()
}

func (a *App) SaveViewSettings(tab string, values map[string]string) error {
	if a.viewHooks == nil {
		return nil
	}
	a.viewHooks.saveSettings(tab, values)
	return nil
}

func (a *App) RunViewCommand(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	callCtx, cancel := context.WithTimeout(context.Background(), desktopViewTimeout)
	defer cancel()
	corehook.ApplyAction(callCtx, HookViewCommand, pkgplugin.CommandHook{ID: id})
	return nil
}
