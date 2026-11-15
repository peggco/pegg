package config

import "testing"

func findProvider(cfg *Config, name string) *LLMConfig {
	for i := range cfg.Providers {
		if cfg.Providers[i].Provider == name {
			return &cfg.Providers[i]
		}
	}
	return nil
}

func TestDefaultConfigHasNoProviders(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Providers) != 0 {
		t.Fatalf("default config providers = %+v, want none until setup", cfg.Providers)
	}
	if findProvider(cfg, "opencode-zen") != nil {
		t.Fatal("default config should not seed a placeholder provider")
	}
}

func TestDefaultConfigHasStallEnabled(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Stall == nil {
		t.Fatal("default config should include stall settings")
	}
	if !cfg.Stall.EnabledValue() {
		t.Fatal("stall should be enabled by default")
	}
	if cfg.Stall.TimeoutSecs != DefaultStallTimeoutSecs {
		t.Fatalf("stall timeout = %d, want %d", cfg.Stall.TimeoutSecs, DefaultStallTimeoutSecs)
	}
	if cfg.Stall.MaxRetries != 0 {
		t.Fatalf("stall max_retries = %d, want 0", cfg.Stall.MaxRetries)
	}
}

func TestUpsertProviderAdds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertProvider(LLMConfig{Provider: "groq", APIKey: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertProvider(LLMConfig{Provider: "openai", APIKey: "b"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("providers = %+v, want 2 (groq + openai)", cfg.Providers)
	}
	if findProvider(cfg, "groq") == nil || findProvider(cfg, "openai") == nil {
		t.Fatalf("added providers missing: %+v", cfg.Providers)
	}
}

func TestUpsertProviderUpdates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertProvider(LLMConfig{Provider: "groq", APIKey: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertProvider(LLMConfig{Provider: "groq", APIKey: "b"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("providers = %+v, want 1 (groq)", cfg.Providers)
	}
	if p := findProvider(cfg, "groq"); p == nil || p.APIKey != "b" {
		t.Fatalf("groq provider = %+v, want API key b", p)
	}
}

func TestRemoveProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertProvider(LLMConfig{Provider: "groq", APIKey: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertProvider(LLMConfig{Provider: "openai", APIKey: "b"}); err != nil {
		t.Fatal(err)
	}

	if err := RemoveProvider("groq"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if findProvider(cfg, "groq") != nil {
		t.Fatalf("groq should be removed: %+v", cfg.Providers)
	}
	if findProvider(cfg, "openai") == nil {
		t.Fatalf("openai should remain: %+v", cfg.Providers)
	}
	if findProvider(cfg, "opencode-zen") != nil {
		t.Fatalf("no placeholder provider should be seeded: %+v", cfg.Providers)
	}

	if err := RemoveProvider("nope"); err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestUpsertMCPServerAdds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertMCPServer("db", MCPServerConfig{Command: "npx", Args: []string{"-y"}}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertMCPServer("remote", MCPServerConfig{URL: "https://example.com/sse"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 2 {
		t.Fatalf("mcp servers = %+v, want 2", cfg.MCPServers)
	}
	if cfg.MCPServers["db"].Command != "npx" {
		t.Fatalf("db = %+v", cfg.MCPServers["db"])
	}
	if cfg.MCPServers["remote"].URL != "https://example.com/sse" {
		t.Fatalf("remote = %+v", cfg.MCPServers["remote"])
	}
}

func TestUpsertMCPServerUpdates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertMCPServer("db", MCPServerConfig{Command: "npx"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertMCPServer("db", MCPServerConfig{Command: "go", Args: []string{"run", "server"}}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("mcp servers = %+v, want 1", cfg.MCPServers)
	}
	if cfg.MCPServers["db"].Command != "go" || len(cfg.MCPServers["db"].Args) != 2 {
		t.Fatalf("db = %+v", cfg.MCPServers["db"])
	}
}

func TestRemoveMCPServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := UpsertMCPServer("db", MCPServerConfig{Command: "npx"}); err != nil {
		t.Fatal(err)
	}
	if err := UpsertMCPServer("remote", MCPServerConfig{URL: "https://example.com/sse"}); err != nil {
		t.Fatal(err)
	}

	if err := RemoveMCPServer("db"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 1 || cfg.MCPServers["remote"].URL == "" {
		t.Fatalf("mcp servers = %+v, want only remote", cfg.MCPServers)
	}

	if err := RemoveMCPServer("nope"); err == nil {
		t.Fatal("expected error for missing server")
	}
}

func TestSystemFirstOnlyDefaultsToOff(t *testing.T) {
	if (LLMConfig{}).SystemFirstOnlyValue() {
		t.Fatal("system_first_only must be off by default")
	}
	off := false
	if (LLMConfig{SystemFirstOnly: &off}).SystemFirstOnlyValue() {
		t.Fatal("system_first_only=false must stay off")
	}
	on := true
	if !(LLMConfig{SystemFirstOnly: &on}).SystemFirstOnlyValue() {
		t.Fatal("system_first_only=true must be on")
	}
}

func TestSystemFirstOnlySurvivesConfigRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	on := true
	if err := UpsertProvider(LLMConfig{Provider: "strict", APIKey: "a", SystemFirstOnly: &on}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p := findProvider(cfg, "strict")
	if p == nil {
		t.Fatal("provider not found after load")
	}
	if !p.SystemFirstOnlyValue() {
		t.Fatal("system_first_only was not persisted")
	}
}

func TestLoadNormalizesLegacyTeamSTTDriver(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultConfig()
	cfg.STT = &STTConfig{
		Provider: "pegg-team",
		Model:    "whisper-1",
		Providers: []STTProviderConfig{{
			Provider: "pegg-team",
			APIKey:   "test-key",
			BaseURL:  "https://team.example.com/v1",
		}},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.STT.Providers) != 1 || loaded.STT.Providers[0].Driver != "openai" {
		t.Fatalf("stt providers = %+v", loaded.STT.Providers)
	}
}

func TestDefaultConfigNeedsProviderSetup(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Onboarding == nil {
		t.Fatal("default config should include onboarding settings")
	}
	if cfg.Onboarding.ProviderSetupDismissed {
		t.Fatal("provider setup should not be dismissed by default")
	}
	if HasConfiguredProvider(cfg) {
		t.Fatal("default config should not report a configured provider")
	}
	if !NeedsProviderSetup(cfg) {
		t.Fatal("default config should need provider setup")
	}
}

func TestHasConfiguredProvider(t *testing.T) {
	tests := []struct {
		name      string
		providers []LLMConfig
		want      bool
	}{
		{
			name:      "seeded placeholder",
			providers: []LLMConfig{{Provider: "opencode-zen"}},
			want:      false,
		},
		{
			name:      "no providers",
			providers: nil,
			want:      false,
		},
		{
			name:      "api key",
			providers: []LLMConfig{{Provider: "openai", APIKey: "sk-test"}},
			want:      true,
		},
		{
			name:      "base url",
			providers: []LLMConfig{{Provider: "opencode-zen", BaseURL: "https://example.com/v1"}},
			want:      true,
		},
		{
			name:      "headers",
			providers: []LLMConfig{{Provider: "custom", Headers: map[string]string{"X-Test": "1"}}},
			want:      true,
		},
		{
			name:      "declared models",
			providers: []LLMConfig{{Provider: "local", Models: []Model{{ID: "m"}}}},
			want:      true,
		},
		{
			name:      "pegg forever with key",
			providers: []LLMConfig{{Provider: "pegg-forever", BaseURL: "https://forever.pegg.dev/v1", APIKey: "vsk_live_test"}},
			want:      true,
		},
		{
			name:      "pegg forever without key",
			providers: []LLMConfig{{Provider: "pegg-forever", BaseURL: "https://forever.pegg.dev/v1"}},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasConfiguredProvider(&Config{Providers: tt.providers}); got != tt.want {
				t.Fatalf("HasConfiguredProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsProviderSetupRespectsDismissal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Onboarding = &OnboardingConfig{ProviderSetupDismissed: true}
	if NeedsProviderSetup(cfg) {
		t.Fatal("dismissed onboarding should not need provider setup")
	}
}

func TestDismissProviderSetupRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := DismissProviderSetup(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Onboarding == nil || !cfg.Onboarding.ProviderSetupDismissed {
		t.Fatalf("onboarding = %+v, want provider setup dismissed", cfg.Onboarding)
	}
	if NeedsProviderSetup(cfg) {
		t.Fatal("dismissal should survive Save/Load")
	}
}

func TestHooksDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Hooks == nil || !cfg.Hooks.Active() || cfg.Hooks.TimeoutSeconds != 5 {
		t.Fatalf("hooks defaults: %#v", cfg.Hooks)
	}

	cfg.Hooks = nil
	normalizeHooks(cfg)
	if cfg.Hooks == nil || cfg.Hooks.TimeoutSeconds != 5 {
		t.Fatalf("normalized: %#v", cfg.Hooks)
	}

	disabled := false
	cfg.Hooks = &HooksConfig{Enabled: &disabled, TimeoutSeconds: 120}
	normalizeHooks(cfg)
	if cfg.Hooks.Active() {
		t.Fatal("expected hooks to be disabled")
	}
	if cfg.Hooks.TimeoutSeconds != 60 {
		t.Fatalf("timeout = %d", cfg.Hooks.TimeoutSeconds)
	}
}
