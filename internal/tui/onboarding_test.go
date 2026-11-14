package tui

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

func newOnboardingTestApp(t *testing.T, cfg *config.Config) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &App{deps: settings.Deps{Config: cfg}, screen: newTestScreen(t)}
}

func TestOnboardingAppearsWithoutConfiguredProvider(t *testing.T) {
	a := newOnboardingTestApp(t, nil)
	a.maybeShowProviderOnboarding()

	if _, ok := a.getOverlay().(*providerOnboarding); !ok {
		t.Fatalf("overlay is %T, want *providerOnboarding", a.getOverlay())
	}
	if !a.onboardingActive.Load() {
		t.Fatal("onboardingActive should be true")
	}
}

func TestOnboardingAbsentWhenDismissed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Onboarding.ProviderSetupDismissed = true
	a := newOnboardingTestApp(t, cfg)
	a.maybeShowProviderOnboarding()

	if a.getOverlay() != nil {
		t.Fatalf("overlay is %T, want nil", a.getOverlay())
	}
	if a.onboardingActive.Load() {
		t.Fatal("onboardingActive should be false")
	}
}

func TestOnboardingHasThreeChoices(t *testing.T) {
	a := newOnboardingTestApp(t, nil)
	a.maybeShowProviderOnboarding()
	o, ok := a.getOverlay().(*providerOnboarding)
	if !ok {
		t.Fatalf("overlay is %T, want *providerOnboarding", a.getOverlay())
	}
	items := o.list.Items()
	if len(items) != 3 {
		t.Fatalf("item count = %d, want 3", len(items))
	}
	want := []string{i18n.T("onboarding.login"), i18n.T("onboarding.apiKey"), i18n.T("onboarding.configure")}
	for i, label := range want {
		if items[i].Label != label {
			t.Errorf("item %d label = %q, want %q", i, items[i].Label, label)
		}
	}
}

func TestOnboardingEscIgnoredOnChoices(t *testing.T) {
	a := newOnboardingTestApp(t, nil)
	a.maybeShowProviderOnboarding()
	o := a.getOverlay().(*providerOnboarding)

	if !o.HandleKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0)) {
		t.Fatal("Esc should be consumed on the choice screen")
	}
	if o.state != onboardingChoice {
		t.Fatalf("state = %v, want onboardingChoice", o.state)
	}
	if a.getOverlay() != o {
		t.Fatal("Esc should not close the onboarding overlay")
	}
}

func TestOnboardingConfigureMyselfOpensSettings(t *testing.T) {
	a := newOnboardingTestApp(t, nil)
	a.maybeShowProviderOnboarding()
	o := a.getOverlay().(*providerOnboarding)

	o.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	o.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	o.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if _, ok := a.getOverlay().(*settings.Settings); !ok {
		t.Fatalf("overlay is %T, want *settings.Settings", a.getOverlay())
	}
	if a.onboardingActive.Load() {
		t.Fatal("onboardingActive should be false after configuring manually")
	}
}

func TestOnboardingAPIKeySavesPeggForever(t *testing.T) {
	a := newOnboardingTestApp(t, nil)
	a.maybeShowProviderOnboarding()
	o := a.getOverlay().(*providerOnboarding)

	o.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	o.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if o.state != onboardingAPIKey {
		t.Fatalf("state = %v, want onboardingAPIKey", o.state)
	}

	oldVerify := verifyOnboardingKey
	verifyOnboardingKey = func(_ context.Context, _, _ string, _ *http.Client) error {
		return errors.New("verification failed")
	}
	t.Cleanup(func() { verifyOnboardingKey = oldVerify })

	for _, r := range "vsk_test_key" {
		o.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
	o.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	for _, p := range cfg.Providers {
		if p.Provider != pegglogin.ProviderName {
			continue
		}
		if p.APIKey != "vsk_test_key" {
			t.Fatalf("api key = %q, want vsk_test_key", p.APIKey)
		}
		if p.BaseURL != pegglogin.DefaultBaseURL {
			t.Fatalf("base url = %q, want %q", p.BaseURL, pegglogin.DefaultBaseURL)
		}
		return
	}
	t.Fatal("pegg-forever provider was not saved")
}
