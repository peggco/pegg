package connect

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestProviderUpsertSystemFirstOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{{Provider: "strict", APIKey: "old-key"}}
	if cfg.Connect == nil {
		cfg.Connect = &config.ConnectConfig{}
	}
	cfg.Connect.AutoApprove = true
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	body := []byte(`{"provider":"strict","api_key":"new-key","system_first_only":true}`)
	if _, err := c.handleSettingsProviderUpsert(context.Background(), &Request{Params: body}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	stored, ok := config.GetProvider("strict")
	if !ok {
		t.Fatal("provider not stored")
	}
	if !stored.SystemFirstOnlyValue() {
		t.Fatal("system_first_only was not applied")
	}
	if stored.APIKey != "new-key" {
		t.Fatalf("api key = %q, want new-key", stored.APIKey)
	}
}

func TestProviderUpsertKeepsSystemFirstOnlyWhenNotSent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enabled := true
	cfg := config.DefaultConfig()
	cfg.Providers = []config.LLMConfig{{Provider: "strict", APIKey: "old-key", SystemFirstOnly: &enabled}}
	if cfg.Connect == nil {
		cfg.Connect = &config.ConnectConfig{}
	}
	cfg.Connect.AutoApprove = true
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	body := []byte(`{"provider":"strict","api_key":"new-key"}`)
	if _, err := c.handleSettingsProviderUpsert(context.Background(), &Request{Params: body}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	stored, ok := config.GetProvider("strict")
	if !ok {
		t.Fatal("provider not stored")
	}
	if !stored.SystemFirstOnlyValue() {
		t.Fatal("system_first_only must be kept when the client does not send it")
	}
}
