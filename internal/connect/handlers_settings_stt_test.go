package connect

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

func TestSTTSettingsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.STT = &config.STTConfig{
		Providers: []config.STTProviderConfig{{Provider: "assemblyai", APIKey: "test-key"}},
	}
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	got, err := c.handleSTTGet(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.(map[string]any)["model"] != "" {
		t.Fatalf("default model = %v", got.(map[string]any)["model"])
	}

	body := []byte(`{"provider":"assemblyai","model":"universal-3-6-pro"}`)
	if _, err := c.handleSTTUpdate(context.Background(), &Request{Params: body}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if c.deps.Config.STT == nil || c.deps.Config.STT.Provider != "assemblyai" || c.deps.Config.STT.Model != "universal-3-6-pro" {
		t.Fatalf("config not updated: %+v", c.deps.Config.STT)
	}
	entry, ok := c.deps.Config.STT.ProviderConfig("assemblyai")
	if !ok || entry.APIKey != "test-key" {
		t.Fatalf("provider entry = %+v", entry)
	}
}
