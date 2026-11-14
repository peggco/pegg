package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/pegglogin"
)

func stubForeverVerify(t *testing.T, fn func(context.Context, string, string, *http.Client) error) {
	t.Helper()
	original := foreverVerify
	foreverVerify = fn
	t.Cleanup(func() { foreverVerify = original })
}

func TestForeverTokenSavesProviderAndPublishesEvent(t *testing.T) {
	c, bus := newTestCLI(t)
	registerTestProvider(t, pegglogin.ProviderName, llm.Model{ID: "pegg-test-model"})
	stubForeverVerify(t, func(context.Context, string, string, *http.Client) error { return nil })

	added := make(chan config.LLMConfig, 1)
	if err := bus.Subscribe(event.TopicProviderAdded, func(cfg config.LLMConfig) { added <- cfg }); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	c.root.SetOut(&buf)
	if err := c.Execute([]string{"login-pegg", "--token", "vsk_live_test"}); err != nil {
		t.Fatal(err)
	}

	select {
	case cfg := <-added:
		if cfg.Provider != pegglogin.ProviderName || cfg.APIKey != "vsk_live_test" {
			t.Fatalf("unexpected event cfg: %+v", cfg)
		}
	default:
		t.Fatal("provider.added event not published")
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	provider := lookupProvider(loaded, pegglogin.ProviderName)
	if provider == nil {
		t.Fatalf("pegg-forever provider not saved: %+v", loaded.Providers)
	}
	if provider.APIKey != "vsk_live_test" {
		t.Fatalf("api key = %q, want vsk_live_test", provider.APIKey)
	}
	if provider.Driver != "openai" {
		t.Fatalf("driver = %q, want openai", provider.Driver)
	}
	if provider.BaseURL != pegglogin.DefaultBaseURL {
		t.Fatalf("base url = %q, want %q", provider.BaseURL, pegglogin.DefaultBaseURL)
	}
	if provider.Timeout != pegglogin.DefaultTimeout {
		t.Fatalf("timeout = %d, want %d", provider.Timeout, pegglogin.DefaultTimeout)
	}

	out := buf.String()
	if !strings.Contains(out, pegglogin.ProviderName) || !strings.Contains(out, pegglogin.DefaultBaseURL) {
		t.Fatalf("output = %q, want provider name and base URL", out)
	}
}

func TestForeverVerificationFailureStillSaves(t *testing.T) {
	c, _ := newTestCLI(t)
	registerTestProvider(t, pegglogin.ProviderName, llm.Model{ID: "pegg-test-model"})
	stubForeverVerify(t, func(context.Context, string, string, *http.Client) error {
		return errors.New("invalid api key")
	})

	var buf bytes.Buffer
	c.root.SetOut(&buf)
	if err := c.Execute([]string{"login-pegg", "--token", "vsk_live_bad"}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "invalid api key") {
		t.Fatalf("output = %q, want verification warning", out)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	provider := lookupProvider(loaded, pegglogin.ProviderName)
	if provider == nil {
		t.Fatalf("pegg-forever provider not saved: %+v", loaded.Providers)
	}
	if provider.APIKey != "vsk_live_bad" {
		t.Fatalf("api key = %q, want vsk_live_bad", provider.APIKey)
	}
}

func TestForeverCommandRegisteredWithAliases(t *testing.T) {
	c, _ := newTestCLI(t)

	var forever *cobra.Command
	for _, cmd := range c.commands.Apply(nil) {
		if cmd.Name() == "login-pegg" {
			forever = cmd
			break
		}
	}
	if forever == nil {
		t.Fatal("login-pegg command not registered")
	}

	aliases := make(map[string]bool, len(forever.Aliases))
	for _, alias := range forever.Aliases {
		aliases[alias] = true
	}
	for _, want := range []string{"forever", "pegg-login"} {
		if !aliases[want] {
			t.Fatalf("alias %q missing from %v", want, forever.Aliases)
		}
	}
	for _, flag := range []string{"token", "no-browser", "port", "device"} {
		if forever.Flags().Lookup(flag) == nil {
			t.Fatalf("flag --%s not registered", flag)
		}
	}
}

func lookupProvider(cfg *config.Config, name string) *config.LLMConfig {
	for i := range cfg.Providers {
		if cfg.Providers[i].Provider == name {
			return &cfg.Providers[i]
		}
	}
	return nil
}
