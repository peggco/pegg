package tui

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

func (o *providerOnboarding) beginDefaults(lc config.LLMConfig) {
	o.pending = lc
	o.state = onboardingDefaultsLoading
	o.message = i18n.T("onboarding.defaultsLoading")
	o.app.requestRedraw()

	go func() {
		d, err := fetchOnboardingDefaults(
			context.Background(),
			pegglogin.DefaultsEndpoint(""),
			nil,
		)
		o.app.postUI(func() { o.showDefaults(d, err) })
	}()
}

func (o *providerOnboarding) showDefaults(d *pegglogin.Defaults, err error) {
	if err != nil || d == nil {
		o.finish(o.pending)
		return
	}
	peggCfg, loadErr := config.Load()
	if loadErr != nil {
		o.finish(o.pending)
		return
	}
	report := pegglogin.ApplyDefaults(peggCfg, d, pegglogin.ProviderName)
	if report.Empty() {
		o.finish(o.pending)
		return
	}

	o.defaults = d
	o.defaultsReport = report
	l := components.NewList("")
	l.SetItems([]components.ListItem{
		{Label: i18n.T("onboarding.defaultsApply"), Detail: i18n.T("onboarding.defaultsApplyDetail")},
		{Label: i18n.T("onboarding.defaultsSkip"), Detail: i18n.T("onboarding.defaultsSkipDetail")},
	})
	l.SetOnSelect(func(index int, _ components.ListItem) {
		if index == 0 {
			o.applyDefaults()
			return
		}
		o.finish(o.pending)
	})
	o.defaultsList = l
	o.state = onboardingDefaults
	o.message = ""
	o.app.requestRedraw()
}

func (o *providerOnboarding) applyDefaults() {
	if o.defaults != nil {
		if err := config.Update(func(cfg *config.Config) error {
			pegglogin.ApplyDefaults(cfg, o.defaults, pegglogin.ProviderName)
			return nil
		}); err != nil {
			o.message = i18n.Tf("onboarding.error", err.Error())
			o.app.requestRedraw()
			return
		}
	}
	o.finish(o.pending)
}

func (o *providerOnboarding) drawDefaults(s tcell.Screen, inner layout.Region, th styles.Theme, bg tcell.Style) {
	lines := []string{i18n.T("onboarding.defaultsSubtitle")}
	for _, change := range o.defaultsReport.Changes {
		lines = append(lines, "• "+change)
	}
	if len(o.defaultsReport.Skipped) > 0 {
		lines = append(lines, i18n.T("onboarding.defaultsSkipped"))
		for _, skipped := range o.defaultsReport.Skipped {
			lines = append(lines, "• "+skipped)
		}
	}

	listHeight := 4
	maxLines := inner.Height - listHeight - 2
	if maxLines < 0 {
		maxLines = 0
	}
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "…")
	}
	for i, line := range lines {
		style := bg.Foreground(th.Foreground)
		if strings.HasPrefix(line, "•") {
			style = bg.Foreground(th.Hint)
		}
		components.DrawText(s, inner.Left, inner.Top+i, components.TruncateTo(line, inner.Width), style)
	}

	listTop := inner.Top + len(lines) + 1
	if listTop+listHeight > inner.Bottom() {
		listHeight = inner.Bottom() - listTop
	}
	if listHeight < 1 {
		listHeight = 1
	}
	o.defaultsRegion = layout.Region{
		Left:   inner.Left,
		Top:    listTop,
		Width:  inner.Width,
		Height: listHeight,
	}
	if o.defaultsList != nil {
		o.defaultsList.Draw(s, o.defaultsRegion, true)
	}
	if o.message != "" {
		components.DrawText(s, inner.Left, inner.Bottom()-2, components.TruncateTo(o.message, inner.Width),
			bg.Foreground(th.Error))
	}
	components.DrawFooter(s, inner, i18n.T("common.footer.list"))
}
