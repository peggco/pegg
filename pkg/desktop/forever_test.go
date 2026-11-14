//go:build gui

package main

import (
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestIsPeggForeverProviderName(t *testing.T) {
	if !isPeggForeverProviderName("pegg-forever") {
		t.Fatal("pegg-forever should match")
	}
	if !isPeggForeverProviderName("  pegg-forever  ") {
		t.Fatal("padded pegg-forever should match")
	}
	for _, name := range []string{"pegg-forever-team", "pegg-team", "openai", ""} {
		if isPeggForeverProviderName(name) {
			t.Fatalf("%q should not match", name)
		}
	}
}

func TestParseForeverLogLine(t *testing.T) {
	url := "https://pegg.dev/app/pegg-forever?state=abc&challenge=def"
	update := parseForeverLogLine(url)
	if update.state != "waiting" || update.url != url {
		t.Fatalf("url update = %+v", update)
	}

	if update := parseForeverLogLine("Waiting for browser approval..."); update.state != "" {
		t.Fatalf("waiting update = %+v", update)
	}

	if update := parseForeverLogLine("Logged in to Pegg"); update.state != "done" {
		t.Fatalf("done update = %+v", update)
	}

	update = parseForeverLogLine("cli: pegg login: timed out waiting for approval")
	if update.state != "error" || update.err == "" {
		t.Fatalf("error update = %+v", update)
	}

	if update := parseForeverLogLine("provider: pegg-forever"); update.state != "" || update.url != "" || update.err != "" {
		t.Fatalf("plain update = %+v", update)
	}
}

func TestProviderSetup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := &App{}
	info, err := a.ProviderSetup()
	if err != nil {
		t.Fatalf("provider setup: %v", err)
	}
	if info.Configured {
		t.Fatal("default config should not be configured")
	}
	if info.Dismissed {
		t.Fatal("default config should not be dismissed")
	}

	if err := a.DismissProviderSetup(); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	info, err = a.ProviderSetup()
	if err != nil {
		t.Fatalf("provider setup after dismiss: %v", err)
	}
	if info.Configured || !info.Dismissed {
		t.Fatalf("after dismiss = %+v", info)
	}

	if err := config.UpsertProvider(config.LLMConfig{Provider: "pegg-forever", APIKey: "vsk_live_test"}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	info, err = a.ProviderSetup()
	if err != nil {
		t.Fatalf("provider setup after upsert: %v", err)
	}
	if !info.Configured {
		t.Fatalf("after upsert = %+v", info)
	}
}

func TestPeggStatusMergesConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := &App{}
	st := a.PeggStatus()
	if st.State != "idle" {
		t.Fatalf("idle state = %q", st.State)
	}
	if st.Configured || st.Provider != "" {
		t.Fatalf("empty status = %+v", st)
	}

	if err := config.UpsertProvider(config.LLMConfig{Provider: "pegg-forever", APIKey: "vsk_live_test"}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	st = a.PeggStatus()
	if !st.Configured || st.Provider != "pegg-forever" {
		t.Fatalf("configured status = %+v", st)
	}
}

func TestPeggLogoutRemovesProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := &App{}
	if err := config.UpsertProvider(config.LLMConfig{Provider: "pegg-forever", APIKey: "vsk_live_test"}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	st, err := a.PeggLogout()
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if st.Configured || st.Provider != "" {
		t.Fatalf("status after logout = %+v", st)
	}
	if _, ok := config.GetProvider("pegg-forever"); ok {
		t.Fatal("pegg-forever should be removed")
	}
}
