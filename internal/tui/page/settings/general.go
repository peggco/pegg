package settings

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/llm/subscription"
	"github.com/peggco/pegg/internal/router"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

type generalTab struct {
	settings *Settings
	index    int
}

func newGeneral(s *Settings) *generalTab { return &generalTab{settings: s} }

const generalRowCount = 7

func (g *generalTab) rowEnabled(i int) bool {
	if i == 5 {
		return g.settings.modelSupportsReasoning()
	}
	return true
}

func (g *generalTab) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if g.index > 0 {
			g.index--
			return true
		}
		return false
	case tcell.KeyDown:
		if g.index < generalRowCount-1 {
			g.index++
			return true
		}
		return false
	case tcell.KeyEnter:
		if !g.rowEnabled(g.index) {
			return true
		}
		g.rows()[g.index].action()
		return true
	}
	return false
}

func (g *generalTab) HandleMouse(x, y int, bounds layout.Region, buttons tcell.ButtonMask) bool {
	rows := g.rows()
	idx := y - bounds.Top
	if idx < 0 || idx >= len(rows) {
		return false
	}
	g.index = idx
	if buttons&tcell.ButtonPrimary != 0 && g.rowEnabled(idx) {
		rows[idx].action()
	}
	return true
}

type genRow struct {
	label  string
	value  string
	action func()
}

func (g *generalTab) rows() []genRow {
	s := g.settings
	effort := s.reasoningEffort
	if effort == "" {
		effort = i18n.T("settings.general.effortDefault")
	}
	return []genRow{
		{label: i18n.T("settings.general.provider"), value: s.providerValue(), action: s.openProviders},
		{label: i18n.T("settings.general.model"), value: s.modelDisplay(), action: s.openModels},
		{label: i18n.T("settings.general.sttProvider"), value: s.sttProviderValue(), action: s.openSTTProviders},
		{label: i18n.T("settings.general.sttModel"), value: s.sttModelValue(), action: s.openSTTModels},
		{label: i18n.T("settings.general.theme"), value: styles.Name(), action: s.openThemes},
		{label: i18n.T("settings.general.reasoning"), value: effort, action: s.openReasoning},
		{label: i18n.T("settings.general.language"), value: i18n.DisplayName(i18n.Current()), action: s.openLanguages},
	}
}

func (g *generalTab) Draw(screen tcell.Screen, bounds layout.Region, _ bool) {
	th := styles.Current()
	rows := g.rows()
	for i, r := range rows {
		y := bounds.Top + i
		style := th.Base().Background(th.InputBg)
		marker := "  "
		enabled := g.rowEnabled(i)
		if i == g.index {
			style = th.Base().Foreground(th.InputText).Background(th.Selection)
			marker = "> "
		} else if !enabled {
			style = th.Base().Foreground(th.Placeholder).Background(th.InputBg)
		}
		label := r.label
		if !enabled {
			label += " " + i18n.T("settings.general.notSupported")
		}
		components.DrawText(screen, bounds.Left+1, y, marker+label, style)
		if enabled || i == g.index {
			components.DrawText(screen, bounds.Right()-len(r.value)-3, y, r.value, style)
		}
	}
}

func (s *Settings) configured(name string) (config.LLMConfig, bool) {
	if s.deps.Config == nil {
		return config.LLMConfig{}, false
	}
	for _, p := range s.deps.Config.Providers {
		if p.Provider == name {
			return p, true
		}
	}
	return config.LLMConfig{}, false
}

func (s *Settings) providerValue() string {
	if s.deps.Config == nil || len(s.deps.Config.Providers) == 0 {
		return i18n.T("settings.provider.none")
	}
	first := s.deps.Config.Providers[0].Provider
	if len(s.deps.Config.Providers) > 1 {
		return i18n.Tf("settings.provider.plus", first, len(s.deps.Config.Providers)-1)
	}
	return first
}

func (s *Settings) openProviders() {
	l := components.NewList(i18n.T("settings.provider.selectTitle"))
	l.SetSearchable(true)
	var items []components.ListItem
	for _, name := range llm.ListProviders() {
		detail, marked := i18n.T("settings.provider.notConfigured"), false
		if _, ok := s.configured(name); ok {
			detail, marked = i18n.T("settings.provider.configured"), true
		}
		items = append(items, components.ListItem{Label: name, Detail: detail, Marked: marked, Data: name})
	}
	l.SetItems(items)
	l.SetOnSelect(func(_ int, item components.ListItem) {
		name, _ := item.Data.(string)
		if info, ok := subscription.Get(name); ok {
			s.openSubscription(name, info)
			return
		}
		if _, ok := s.configured(name); ok {
			s.openProviderChoice(name)
		} else {
			s.openAPIKey(name)
		}
	})
	s.openSub(&listModal{title: i18n.T("settings.provider.selectSearch"), list: l, onBack: s.back})
}

func (s *Settings) openSubscription(name string, info subscription.Info) {
	if !info.Status().LoggedIn {
		l := components.NewList(i18n.T("settings.provider.signInRequired"))
		l.SetItems([]components.ListItem{
			{Label: info.Hint, Disabled: true},
			{Label: i18n.T("settings.provider.retry"), Detail: i18n.T("settings.provider.retryDetail")},
			{Label: i18n.T("settings.provider.cancel"), Detail: i18n.T("settings.provider.cancelDetail")},
		})
		l.SelectFirstEnabled()
		l.SetOnSelect(func(i int, _ components.ListItem) {
			if i == 1 {
				s.openSubscription(name, info)
			} else {
				s.back()
			}
		})
		s.openSub(&listModal{title: i18n.Tf("settings.provider.notSignedIn", name), list: l, onBack: s.back})
		return
	}

	cfg := config.LLMConfig{Provider: name}
	if err := config.UpsertProvider(cfg); err != nil {
		s.errMsg = i18n.Tf("settings.provider.saveError", err.Error())
		s.back()
		return
	}
	if fresh, err := config.Load(); err == nil {
		s.deps.Config = fresh
	}
	if s.deps.Bus != nil {
		s.deps.Bus.Publish(event.TopicProviderAdded, cfg)
	}
	s.errMsg = ""
	s.back()
}

func (s *Settings) openProviderChoice(name string) {
	l := components.NewList(i18n.Tf("settings.provider.choiceTitle", name))
	l.SetItems([]components.ListItem{
		{Label: i18n.T("settings.provider.useExisting"), Detail: i18n.T("settings.provider.useExistingDetail")},
		{Label: i18n.T("settings.provider.configureAgain"), Detail: i18n.T("settings.provider.configureAgainDetail")},
	})
	l.SetOnSelect(func(i int, _ components.ListItem) {
		if i == 1 {
			s.openAPIKey(name)
		} else {
			s.back()
		}
	})
	s.openSub(&listModal{title: i18n.T("settings.provider.alreadyConfigured"), list: l, onBack: s.back})
}

func (s *Settings) openAPIKey(name string) {
	f := components.NewField(i18n.Tf("settings.provider.apiKeyTitle", name))
	f.SetMask('*')
	f.SetPlaceholder(i18n.T("settings.provider.apiKeyPlaceholder"))
	f.Focus()
	f.SetOnEnter(func(key string) { s.saveProvider(name, strings.TrimSpace(key)) })
	f.SetOnCancel(s.back)
	s.openSub(&fieldModal{title: i18n.T("settings.provider.configureTitle"), field: f})
}

func (s *Settings) saveProvider(name, apiKey string) {
	cfg, ok := s.configured(name)
	if !ok {
		cfg = config.LLMConfig{Provider: name}
	}
	cfg.APIKey = apiKey

	if err := config.UpsertProvider(cfg); err != nil {
		s.errMsg = i18n.Tf("settings.provider.saveError", err.Error())
		s.back()
		return
	}
	s.errMsg = ""

	if fresh, err := config.Load(); err == nil {
		s.deps.Config = fresh
	}
	if s.deps.Bus != nil {
		s.deps.Bus.Publish(event.TopicProviderAdded, cfg)
	}
	s.back()
}

func (s *Settings) modelDisplay() string {
	if s.model.provider == router.SmartRouterModel {
		return i18n.T("settings.model.smartRouter")
	}
	if s.model.provider == "" {
		return i18n.T("settings.model.none")
	}
	name := s.model.model.ID
	if s.model.model.Name != "" {
		name = s.model.model.Name
	}
	return s.model.provider + "/" + name
}

type modelData struct {
	provider string
	model    llm.Model
}

func (s *Settings) openModels() {
	l := components.NewList(i18n.T("settings.model.selectTitle"))
	l.SetSearchable(true)

	var items []components.ListItem
	for _, opt := range router.ModelOptionsHook.Apply(nil) {
		items = append(items, components.ListItem{
			Label:  opt.Label,
			Detail: opt.Detail,
			Marked: s.model.provider == opt.Provider,
			Data:   modelData{provider: opt.Provider, model: llm.Model{ID: opt.ModelID, Name: opt.Label}},
		})
	}
	if s.deps.Config != nil && s.deps.LLM != nil {
		for _, p := range s.deps.Config.Providers {
			models, err := s.deps.LLM.Models(p.Provider)
			if err != nil {
				continue
			}
			for _, m := range models {
				label := m.ID
				if m.Name != "" {
					label = m.Name
				}
				marked := s.model.provider == p.Provider && s.model.model.ID == m.ID
				items = append(items, components.ListItem{
					Label:  label,
					Detail: p.Provider,
					Marked: marked,
					Data:   modelData{provider: p.Provider, model: m},
				})
			}
		}
	}
	l.SetItems(items)
	l.SetOnSelect(func(_ int, item components.ListItem) {
		if md, ok := item.Data.(modelData); ok {
			s.model = selectedModel{provider: md.provider, model: md.model}
			if s.onModelChange != nil {
				s.onModelChange(md.provider, md.model)
			}
		}
		s.back()
	})
	s.openSub(&listModal{title: i18n.T("settings.model.selectSearch"), list: l, onBack: s.back})
}

func (s *Settings) openThemes() {
	l := components.NewList(i18n.T("settings.theme.title"))
	var items []components.ListItem
	for _, name := range styles.Names() {
		items = append(items, components.ListItem{Label: name, Marked: name == styles.Name()})
	}
	l.SetItems(items)
	l.SetOnSelect(func(_ int, item components.ListItem) {
		styles.Set(item.Label)
		if s.onThemeChange != nil {
			s.onThemeChange()
		}
		s.back()
	})
	s.openSub(&listModal{title: i18n.T("settings.theme.title"), list: l, onBack: s.back})
}

func (s *Settings) openLanguages() {
	l := components.NewList(i18n.T("settings.language.title"))
	var items []components.ListItem
	for _, code := range i18n.Names() {
		items = append(items, components.ListItem{Label: i18n.DisplayName(code), Detail: code, Marked: code == i18n.Current(), Data: code})
	}
	l.SetItems(items)
	l.SetOnSelect(func(_ int, item components.ListItem) {
		code, _ := item.Data.(string)
		if i18n.Set(code) {
			if s.onLanguageChange != nil {
				s.onLanguageChange()
			}
			s.rebuildTabs()
		}
		s.back()
	})
	s.openSub(&listModal{title: i18n.T("settings.language.title"), list: l, onBack: s.back})
}
