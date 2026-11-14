package settings

import (
	"context"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/llm/subscription"
	"github.com/peggco/pegg/internal/pegglogin"
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
	var configuredProviders []config.LLMConfig
	if s.deps.Config != nil {
		configuredProviders = s.deps.Config.Providers
	}
	var items []components.ListItem
	for _, name := range llm.ListProviderNames(configuredProviders) {
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
		if name == pegglogin.ProviderName {
			if s.peggForeverConfigured() {
				s.openProviderChoice(name)
			} else {
				s.openPeggForeverChoice()
			}
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

	cfg, _ := config.GetProvider(name)
	cfg.Provider = name
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
	s.notifyProviderChange()
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

var verifyProviderKey = pegglogin.Verify

func (s *Settings) peggForeverConfigured() bool {
	cfg, ok := s.configured(pegglogin.ProviderName)
	return ok && strings.TrimSpace(cfg.APIKey) != ""
}

func (s *Settings) openPeggForeverChoice() {
	l := components.NewList(i18n.Tf("settings.provider.choiceTitle", pegglogin.ProviderName))
	l.SetItems([]components.ListItem{
		{Label: i18n.T("onboarding.login"), Detail: i18n.T("onboarding.loginDetail")},
		{Label: i18n.T("onboarding.apiKey"), Detail: i18n.T("onboarding.apiKeyDetail")},
	})
	l.SetOnSelect(func(i int, _ components.ListItem) {
		if i == 1 {
			s.openAPIKey(pegglogin.ProviderName)
			return
		}
		s.startPeggLogin()
	})
	s.openSub(&listModal{title: i18n.T("settings.provider.configureTitle"), list: l, onBack: s.back})
}

func (s *Settings) startPeggLogin() {
	ctx, cancel := context.WithCancel(context.Background())
	gen := s.beginPeggLogin(cancel)
	m := &peggLoginModal{
		settings: s,
		message:  i18n.T("onboarding.waiting"),
	}
	s.openSub(m)
	s.redraw()

	go func() {
		defer cancel()
		cfg, err := pegglogin.Login(ctx, pegglogin.Options{
			OnURL: func(url string) {
				s.dispatch(func() {
					if !s.loginAlive(gen) {
						return
					}
					m.setURL(url)
					s.redraw()
				})
			},
		})
		if err != nil {
			s.dispatch(func() {
				if !s.completePeggLogin(gen) {
					return
				}
				m.setError(i18n.Tf("onboarding.error", err.Error()))
				s.redraw()
			})
			return
		}
		s.dispatch(func() {
			if !s.completePeggLogin(gen) {
				return
			}
			s.savePeggForeverLogin(cfg)
			s.redraw()
		})
	}()
}

func (s *Settings) savePeggForeverLogin(cfg *pegglogin.Config) {
	if err := pegglogin.Apply(cfg); err != nil {
		s.errMsg = i18n.Tf("onboarding.error", err.Error())
		s.back()
		return
	}
	lc := config.LLMConfig{
		Provider: pegglogin.ProviderName,
		Driver:   cfg.Driver,
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Timeout:  cfg.Timeout,
	}
	s.errMsg = ""
	s.afterProviderSave(lc)
	s.back()
}

func (s *Settings) redraw() {
	if s.requestRedraw != nil {
		s.requestRedraw()
	}
}

type peggLoginModal struct {
	settings *Settings

	mu      sync.Mutex
	url     string
	message string
	errMsg  string

	region    layout.Region
	cancelHit layout.Region
}

func (m *peggLoginModal) setURL(url string) {
	m.mu.Lock()
	m.url = url
	m.mu.Unlock()
}

func (m *peggLoginModal) setError(msg string) {
	m.mu.Lock()
	m.errMsg = msg
	m.mu.Unlock()
}

func (m *peggLoginModal) snapshot() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.url, m.errMsg
}

func (m *peggLoginModal) close() {
	m.settings.cancelPeggLogin()
	m.settings.back()
	m.settings.redraw()
}

func (m *peggLoginModal) HandleKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEsc {
		m.close()
	}
	return true
}

func (m *peggLoginModal) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	if buttons&tcell.ButtonPrimary != 0 && inRegion(m.cancelHit, x, y) {
		m.close()
		return true
	}
	if buttons&tcell.ButtonPrimary != 0 && !inRegion(m.region, x, y) {
		m.close()
	}
	return true
}

func (m *peggLoginModal) Draw(s tcell.Screen, bounds layout.Region, _ bool) {
	th := styles.Current()
	w, h := 64, 12
	if bounds.Width-8 < w {
		w = bounds.Width - 8
	}
	inner := components.DrawCenteredBox(s, bounds, w, h, i18n.T("settings.provider.configureTitle"))
	m.region = inner
	bg := th.Base().Background(th.InputBg)

	url, errMsg := m.snapshot()
	components.DrawText(s, inner.Left, inner.Top, m.message, bg.Foreground(th.Accent))
	if errMsg != "" {
		components.DrawText(s, inner.Left, inner.Top+2, components.TruncateTo(errMsg, inner.Width),
			bg.Foreground(th.Error))
	} else if url != "" {
		components.DrawText(s, inner.Left, inner.Top+2, i18n.T("onboarding.openBrowser"), bg.Foreground(th.Hint))
		components.DrawText(s, inner.Left, inner.Top+3, components.TruncateTo(url, inner.Width),
			bg.Foreground(th.Foreground))
	}

	m.cancelHit = layout.Region{Left: inner.Left, Top: inner.Bottom() - 2, Width: 10, Height: 1}
	components.DrawText(s, m.cancelHit.Left, m.cancelHit.Top, "[ "+i18n.T("onboarding.cancel")+" ]",
		bg.Foreground(th.Accent))
	components.DrawFooter(s, inner, i18n.T("common.enterSaveEscCancel"))
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
	if name == pegglogin.ProviderName {
		cfg.Driver = "openai"
		cfg.BaseURL = pegglogin.DefaultBaseURL
		if cfg.Timeout == 0 {
			cfg.Timeout = pegglogin.DefaultTimeout
		}
	}

	if err := config.UpsertProvider(cfg); err != nil {
		s.errMsg = i18n.Tf("settings.provider.saveError", err.Error())
		s.back()
		return
	}
	s.errMsg = ""
	s.afterProviderSave(cfg)
	s.back()

	if name == pegglogin.ProviderName {
		verify := verifyProviderKey
		baseURL, key := cfg.BaseURL, cfg.APIKey
		go func() {
			if err := verify(context.Background(), baseURL, key, nil); err != nil {
				s.dispatch(func() {
					if s.isClosed() {
						return
					}
					s.errMsg = i18n.Tf("onboarding.verifyWarning", err.Error())
					s.redraw()
				})
			}
		}()
	}
}

func (s *Settings) afterProviderSave(cfg config.LLMConfig) {
	if fresh, err := config.Load(); err == nil {
		s.deps.Config = fresh
	}
	if s.deps.Bus != nil {
		s.deps.Bus.Publish(event.TopicProviderAdded, cfg)
	}
	s.notifyProviderChange()
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
