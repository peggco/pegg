package tui

import (
	"context"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

type onboardingState int

const (
	onboardingChoice onboardingState = iota
	onboardingAPIKey
	onboardingWaiting
	onboardingDefaultsLoading
	onboardingDefaults
	onboardingModelsLoading
)

var verifyOnboardingKey = pegglogin.Verify

var fetchOnboardingDefaults = pegglogin.FetchDefaults

type providerOnboarding struct {
	app   *App
	state onboardingState
	list  *components.List
	field *components.Field

	cancel    context.CancelFunc
	loginID   int
	url       string
	message   string
	inner     layout.Region
	cancelHit layout.Region

	pending        config.LLMConfig
	defaults       *pegglogin.Defaults
	defaultsReport pegglogin.Report
	defaultsList   *components.List
	defaultsRegion layout.Region
}

var _ components.MouseComponent = (*providerOnboarding)(nil)

func newProviderOnboarding(app *App) *providerOnboarding {
	o := &providerOnboarding{app: app}
	o.resetChoices()
	return o
}

func (o *providerOnboarding) resetChoices() {
	l := components.NewList(i18n.T("onboarding.title"))
	l.SetItems([]components.ListItem{
		{Label: i18n.T("onboarding.login"), Detail: i18n.T("onboarding.loginDetail")},
		{Label: i18n.T("onboarding.apiKey"), Detail: i18n.T("onboarding.apiKeyDetail")},
		{Label: i18n.T("onboarding.configure"), Detail: i18n.T("onboarding.configureDetail")},
	})
	l.SetOnSelect(func(i int, _ components.ListItem) { o.choose(i) })
	o.list = l
	o.state = onboardingChoice
}

func (o *providerOnboarding) choose(index int) {
	switch index {
	case 0:
		o.startLogin()
	case 1:
		o.startAPIKey()
	case 2:
		o.configureMyself()
	}
}

func (o *providerOnboarding) configureMyself() {
	o.app.onboardingActive.Store(false)
	if err := config.DismissProviderSetup(); err != nil {
		o.message = i18n.Tf("onboarding.error", err.Error())
		o.app.requestRedraw()
		return
	}
	o.app.openSettingsProviders()
}

func (o *providerOnboarding) startAPIKey() {
	f := components.NewField(i18n.T("onboarding.apiKeyTitle"))
	f.SetMask('*')
	f.SetPlaceholder(i18n.T("settings.provider.apiKeyPlaceholder"))
	f.Focus()
	f.SetOnEnter(func(key string) { o.saveAPIKey(strings.TrimSpace(key)) })
	f.SetOnCancel(func() {
		o.state = onboardingChoice
		o.message = ""
		o.app.requestRedraw()
	})
	o.field = f
	o.state = onboardingAPIKey
	o.message = ""
	o.app.requestRedraw()
}

func (o *providerOnboarding) saveAPIKey(key string) {
	if key == "" {
		return
	}
	loginCfg := &pegglogin.Config{
		Name:    pegglogin.ProviderName,
		Driver:  "openai",
		BaseURL: pegglogin.DefaultBaseURL,
		APIKey:  key,
		Timeout: pegglogin.DefaultTimeout,
	}
	if err := pegglogin.Apply(loginCfg); err != nil {
		o.message = i18n.Tf("onboarding.error", err.Error())
		o.app.requestRedraw()
		return
	}
	lc := config.LLMConfig{
		Provider: pegglogin.ProviderName,
		Driver:   loginCfg.Driver,
		BaseURL:  loginCfg.BaseURL,
		APIKey:   loginCfg.APIKey,
		Timeout:  loginCfg.Timeout,
	}
	o.beginDefaults(lc)
	verify := verifyOnboardingKey
	go func() {
		if err := verify(context.Background(), loginCfg.BaseURL, loginCfg.APIKey, nil); err != nil {
			o.app.postUI(func() {
				o.app.errorMsg = i18n.Tf("onboarding.verifyWarning", err.Error())
				o.app.requestRedraw()
			})
		}
	}()
}

func (o *providerOnboarding) startLogin() {
	parent := o.app.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	o.cancel = cancel
	o.loginID++
	id := o.loginID
	o.state = onboardingWaiting
	o.url = ""
	o.message = i18n.T("onboarding.waiting")
	o.app.requestRedraw()

	go func() {
		defer cancel()
		cfg, err := pegglogin.Login(ctx, pegglogin.Options{
			OnURL: func(url string) {
				o.app.postUI(func() {
					if o.loginID != id {
						return
					}
					o.url = url
					o.app.requestRedraw()
				})
			},
		})
		o.app.postUI(func() { o.finishLogin(id, cfg, err) })
	}()
}

func (o *providerOnboarding) finishLogin(id int, cfg *pegglogin.Config, err error) {
	if id != o.loginID {
		return
	}
	o.cancel = nil
	if err != nil {
		o.state = onboardingChoice
		o.url = ""
		o.message = i18n.Tf("onboarding.error", err.Error())
		o.app.requestRedraw()
		return
	}
	if err := pegglogin.Apply(cfg); err != nil {
		o.state = onboardingChoice
		o.url = ""
		o.message = i18n.Tf("onboarding.error", err.Error())
		o.app.requestRedraw()
		return
	}
	lc := config.LLMConfig{
		Provider: pegglogin.ProviderName,
		Driver:   cfg.Driver,
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Timeout:  cfg.Timeout,
	}
	o.beginDefaults(lc)
}

func (o *providerOnboarding) finish(lc config.LLMConfig) {
	o.pending = lc
	if o.app.deps.LLM == nil {
		o.closeAfterModels()
		return
	}
	o.app.providerAdded(lc)
	o.state = onboardingModelsLoading
	o.message = i18n.T("onboarding.loadingModels")
	o.app.requestRedraw()

	o.app.reloadConfig()
	o.app.chatMu.Lock()
	providers := []config.LLMConfig{lc}
	if o.app.deps.Config != nil && len(o.app.deps.Config.Providers) > 0 {
		providers = append([]config.LLMConfig(nil), o.app.deps.Config.Providers...)
	}
	o.app.chatMu.Unlock()

	go func() {
		ready := false
		deadline := time.Now().Add(60 * time.Second)
		for {
			if models, err := o.app.deps.LLM.Models(lc.Provider); err == nil && len(models) > 0 {
				ready = true
				break
			}
			if time.Now().After(deadline) {
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			o.app.deps.LLM.EnsureProviders(ctx, providers)
			o.app.deps.LLM.RefreshEmptyProviders(ctx, providers)
			cancel()
			time.Sleep(750 * time.Millisecond)
		}
		o.app.postUI(func() {
			if !ready {
				o.app.errorMsg = i18n.T("onboarding.modelsTimeout")
			}
			o.closeAfterModels()
		})
	}()
}

func (o *providerOnboarding) closeAfterModels() {
	o.app.onboardingActive.Store(false)
	o.app.selectPreferred()
	o.app.setOverlay(nil)
	o.app.requestRedraw()
}

func (o *providerOnboarding) cancelLogin() {
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
	o.loginID++
	o.state = onboardingChoice
	o.url = ""
	o.message = ""
	o.app.requestRedraw()
}

func (o *providerOnboarding) HandleKey(ev *tcell.EventKey) bool {
	switch o.state {
	case onboardingChoice:
		if ev.Key() == tcell.KeyEsc {
			return true
		}
		return o.list.HandleKey(ev)
	case onboardingAPIKey:
		return o.field.HandleKey(ev)
	case onboardingWaiting:
		if ev.Key() == tcell.KeyEsc || ev.Key() == tcell.KeyEnter {
			o.cancelLogin()
		}
		return true
	case onboardingDefaultsLoading:
		if ev.Key() == tcell.KeyEsc {
			o.finish(o.pending)
		}
		return true
	case onboardingDefaults:
		if ev.Key() == tcell.KeyEsc {
			o.finish(o.pending)
			return true
		}
		if o.defaultsList != nil {
			return o.defaultsList.HandleKey(ev)
		}
		return true
	case onboardingModelsLoading:
		return true
	}
	return false
}

func (o *providerOnboarding) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	switch o.state {
	case onboardingChoice:
		return o.list.HandleMouse(x, y, buttons)
	case onboardingAPIKey:
		if o.field.HandleMouse(x, y, buttons) {
			return true
		}
		if buttons&tcell.ButtonPrimary != 0 && !inBounds(o.inner, x, y) {
			o.state = onboardingChoice
			o.message = ""
			o.app.requestRedraw()
		}
		return true
	case onboardingWaiting:
		if buttons&tcell.ButtonPrimary != 0 && inBounds(o.cancelHit, x, y) {
			o.cancelLogin()
			return true
		}
		return true
	case onboardingDefaultsLoading:
		return true
	case onboardingDefaults:
		if o.defaultsList != nil {
			return o.defaultsList.HandleMouse(x, y, buttons)
		}
		return true
	case onboardingModelsLoading:
		return true
	}
	return false
}

func (o *providerOnboarding) Draw(s tcell.Screen, bounds layout.Region, _ bool) {
	th := styles.Current()
	components.DrawModalBackdrop(s, bounds)

	w, h := 64, 16
	title := i18n.T("onboarding.title")
	if o.state == onboardingDefaults || o.state == onboardingDefaultsLoading {
		w, h = 76, 24
		title = i18n.T("onboarding.defaultsTitle")
	}
	if o.state == onboardingModelsLoading {
		title = i18n.T("onboarding.modelsTitle")
	}
	if bounds.Width-8 < w {
		w = bounds.Width - 8
	}
	if bounds.Height-4 < h {
		h = bounds.Height - 4
	}
	inner := components.DrawCenteredBox(s, bounds, w, h, title)
	o.inner = inner
	bg := th.Base().Background(th.InputBg)

	switch o.state {
	case onboardingChoice:
		components.DrawText(s, inner.Left, inner.Top, components.TruncateTo(i18n.T("onboarding.subtitle"), inner.Width),
			bg.Foreground(th.Hint))
		listHeight := inner.Height - 4
		if o.message != "" {
			listHeight = inner.Height - 6
		}
		listRegion := layout.Region{Left: inner.Left, Top: inner.Top + 2, Width: inner.Width, Height: listHeight}
		o.list.Draw(s, listRegion, true)
		if o.message != "" {
			components.DrawText(s, inner.Left, inner.Bottom()-2, components.TruncateTo(o.message, inner.Width),
				bg.Foreground(th.Error))
		}
		components.DrawFooter(s, inner, i18n.T("common.footer.list"))
	case onboardingAPIKey:
		o.field.Draw(s, inner, true)
		if o.message != "" {
			components.DrawText(s, inner.Left, inner.Bottom()-2, components.TruncateTo(o.message, inner.Width),
				bg.Foreground(th.Error))
		}
	case onboardingWaiting:
		components.DrawText(s, inner.Left, inner.Top, i18n.T("onboarding.waiting"), bg.Foreground(th.Accent))
		if o.url != "" {
			components.DrawText(s, inner.Left, inner.Top+2, i18n.T("onboarding.openBrowser"), bg.Foreground(th.Hint))
			components.DrawText(s, inner.Left, inner.Top+3, components.TruncateTo(o.url, inner.Width),
				bg.Foreground(th.Foreground))
		}
		o.cancelHit = layout.Region{Left: inner.Left, Top: inner.Bottom() - 2, Width: 10, Height: 1}
		components.DrawText(s, o.cancelHit.Left, o.cancelHit.Top, "[ "+i18n.T("onboarding.cancel")+" ]",
			bg.Foreground(th.Accent))
		components.DrawFooter(s, inner, i18n.T("common.enterSaveEscCancel"))
	case onboardingDefaultsLoading:
		components.DrawText(s, inner.Left, inner.Top, o.message, bg.Foreground(th.Accent))
		components.DrawFooter(s, inner, i18n.T("common.escCancel"))
	case onboardingDefaults:
		o.drawDefaults(s, inner, th, bg)
	case onboardingModelsLoading:
		components.DrawText(s, inner.Left, inner.Top, o.message, bg.Foreground(th.Accent))
		components.DrawFooter(s, inner, i18n.T("onboarding.loadingModelsHint"))
	}
}

func inBounds(r layout.Region, x, y int) bool {
	return x >= r.Left && x < r.Right() && y >= r.Top && y < r.Bottom()
}
