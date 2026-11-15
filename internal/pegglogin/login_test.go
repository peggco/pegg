package pegglogin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
)

func TestBuildAuthorizeURL(t *testing.T) {
	got := BuildAuthorizeURL(
		"https://pegg.dev/",
		"state-value",
		"challenge-value",
		"http://127.0.0.1:4321/callback",
		"my-laptop",
	)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if u.Scheme != "https" || u.Host != "pegg.dev" {
		t.Fatalf("unexpected origin: %s", u.Scheme+"://"+u.Host)
	}
	if u.Path != "/app/pegg-forever" {
		t.Fatalf("unexpected path: %s", u.Path)
	}
	q := u.Query()
	want := map[string]string{
		"state":     "state-value",
		"challenge": "challenge-value",
		"callback":  "http://127.0.0.1:4321/callback",
		"device":    "my-laptop",
	}
	for key, value := range want {
		if q.Get(key) != value {
			t.Fatalf("query %s = %q, want %q", key, q.Get(key), value)
		}
	}
}

func TestExchangeNormalizesDefaults(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/pegg-forever/cli/exchange" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"config":{"provider":{"api_key":"vsk_live_test"}}}`)
	}))
	defer srv.Close()

	cfg, err := Exchange(context.Background(), srv.URL, "the-code", "the-verifier", srv.Client())
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if body["code"] != "the-code" || body["code_verifier"] != "the-verifier" {
		t.Fatalf("unexpected request body: %#v", body)
	}
	if cfg.APIKey != "vsk_live_test" {
		t.Fatalf("api key = %q", cfg.APIKey)
	}
	if cfg.Name != ProviderName {
		t.Fatalf("name = %q, want %q", cfg.Name, ProviderName)
	}
	if cfg.Driver != "openai" {
		t.Fatalf("driver = %q, want openai", cfg.Driver)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("base url = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Fatalf("timeout = %d, want %d", cfg.Timeout, DefaultTimeout)
	}
}

func TestExchangeUsesReturnedProviderValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"config":{"provider":{"name":"custom","driver":"openai","base_url":"https://example.test/v1","api_key":"vsk_live_test","timeout":42}}}`)
	}))
	defer srv.Close()

	cfg, err := Exchange(context.Background(), srv.URL, "code", "verifier", srv.Client())
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if cfg.Name != "custom" || cfg.BaseURL != "https://example.test/v1" || cfg.Timeout != 42 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoginWithBrowserCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pegg-forever/cli/exchange" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["code"] != "test-code" {
			t.Errorf("code = %q, want test-code", body["code"])
		}
		if body["code_verifier"] == "" {
			t.Error("missing code_verifier")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"config":{"provider":{"api_key":"vsk_live_login"}}}`)
	}))
	defer srv.Close()

	restoreApp, restoreAPI := appURL, appAPIURL
	appURL, appAPIURL = "https://app.test", srv.URL
	defer func() { appURL, appAPIURL = restoreApp, restoreAPI }()

	var opened string
	var callbackErr error
	cfg, err := Login(context.Background(), Options{
		Device: "test-device",
		OnURL:  func(string) {},
		OpenBrowser: func(raw string) error {
			opened = raw
			u, parseErr := url.Parse(raw)
			if parseErr != nil {
				callbackErr = parseErr
				return nil
			}
			cb, parseErr := url.Parse(u.Query().Get("callback"))
			if parseErr != nil {
				callbackErr = parseErr
				return nil
			}
			q := cb.Query()
			q.Set("code", "test-code")
			q.Set("state", u.Query().Get("state"))
			cb.RawQuery = q.Encode()
			resp, getErr := http.Get(cb.String())
			if getErr != nil {
				callbackErr = getErr
				return nil
			}
			resp.Body.Close()
			return nil
		},
	})
	if callbackErr != nil {
		t.Fatalf("callback: %v", callbackErr)
	}
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if cfg.APIKey != "vsk_live_login" {
		t.Fatalf("api key = %q", cfg.APIKey)
	}
	if opened == "" {
		t.Fatal("browser opener was not called")
	}
	u, _ := url.Parse(opened)
	if u.Path != "/app/pegg-forever" {
		t.Fatalf("authorize path = %s", u.Path)
	}
	if u.Query().Get("state") == "" || u.Query().Get("challenge") == "" {
		t.Fatalf("authorize url missing pkce params: %s", opened)
	}
	if u.Query().Get("device") != "test-device" {
		t.Fatalf("device = %q", u.Query().Get("device"))
	}
}

func TestLoginStateMismatch(t *testing.T) {
	_, err := Login(context.Background(), Options{
		Timeout: 2 * time.Second,
		OpenBrowser: func(raw string) error {
			hitCallback(t, raw, "wrong-state", "some-code")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("err = %v, want state mismatch", err)
	}
}

func TestLoginMissingCode(t *testing.T) {
	_, err := Login(context.Background(), Options{
		Timeout: 2 * time.Second,
		OpenBrowser: func(raw string) error {
			hitCallback(t, raw, "", "")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "code") {
		t.Fatalf("err = %v, want missing code", err)
	}
}

func TestLoginCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Login(ctx, Options{NoBrowser: true, Timeout: time.Second})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestApplyWritesProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Apply(&Config{APIKey: "vsk_live_test"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pegg", "pegg.json")); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	peggCfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	provider, ok := findProvider(peggCfg, ProviderName)
	if !ok {
		t.Fatal("pegg-forever provider not saved")
	}
	if provider.Driver != "openai" || provider.BaseURL != DefaultBaseURL {
		t.Fatalf("unexpected provider: %#v", provider)
	}
	if provider.APIKey != "vsk_live_test" || provider.Timeout != DefaultTimeout {
		t.Fatalf("unexpected provider: %#v", provider)
	}
	sttProvider, ok := peggCfg.STT.ProviderConfig(ProviderName)
	if !ok {
		t.Fatal("pegg-forever stt provider not saved")
	}
	if sttProvider.Driver != "openai" || sttProvider.BaseURL != DefaultBaseURL || sttProvider.APIKey != "vsk_live_test" {
		t.Fatalf("unexpected stt provider: %#v", sttProvider)
	}
}

func TestApplyPreservesExistingFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	existing := config.DefaultConfig()
	existing.Providers = append(existing.Providers, config.LLMConfig{
		Provider: ProviderName,
		Driver:   "custom",
		BaseURL:  "https://old.example/v1",
		APIKey:   "old-key",
		Timeout:  5,
		Headers:  map[string]string{"X-Test": "1"},
		Models:   []config.Model{{ID: "kept-model"}},
	})
	existing.STT = &config.STTConfig{
		MaxDurationSecs: config.DefaultSTTMaxDurationSecs,
		Providers: []config.STTProviderConfig{{
			Provider: ProviderName,
			Driver:   "custom",
			BaseURL:  "https://old.example/v1",
			APIKey:   "old-key",
			Models:   []config.STTModel{{ID: "kept-stt"}},
		}},
	}
	if err := config.Save(existing); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := Apply(&Config{APIKey: "new-key"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	peggCfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	provider, ok := findProvider(peggCfg, ProviderName)
	if !ok {
		t.Fatal("pegg-forever provider not saved")
	}
	if provider.APIKey != "new-key" || provider.Driver != "openai" || provider.BaseURL != DefaultBaseURL || provider.Timeout != DefaultTimeout {
		t.Fatalf("provider fields not updated: %#v", provider)
	}
	if provider.Headers["X-Test"] != "1" {
		t.Fatalf("headers not preserved: %#v", provider.Headers)
	}
	if len(provider.Models) != 1 || provider.Models[0].ID != "kept-model" {
		t.Fatalf("models not preserved: %#v", provider.Models)
	}
	sttProvider, ok := peggCfg.STT.ProviderConfig(ProviderName)
	if !ok {
		t.Fatal("pegg-forever stt provider not saved")
	}
	if sttProvider.APIKey != "new-key" || sttProvider.Driver != "openai" || sttProvider.BaseURL != DefaultBaseURL {
		t.Fatalf("stt provider fields not updated: %#v", sttProvider)
	}
	if len(sttProvider.Models) != 1 || sttProvider.Models[0].ID != "kept-stt" {
		t.Fatalf("stt models not preserved: %#v", sttProvider.Models)
	}
}

func TestVerify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s, want /models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer vsk_live_test" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := Verify(context.Background(), srv.URL, "vsk_live_test", srv.Client()); err != nil {
		t.Fatalf("verify: %v", err)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "invalid key")
	}))
	defer failing.Close()

	if err := Verify(context.Background(), failing.URL, "bad", failing.Client()); err == nil {
		t.Fatal("expected verification failure")
	}
}

func hitCallback(t *testing.T, authorizeURL, state, code string) {
	t.Helper()
	u, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	cb, err := url.Parse(u.Query().Get("callback"))
	if err != nil {
		t.Fatalf("parse callback url: %v", err)
	}
	q := cb.Query()
	if state == "" {
		state = u.Query().Get("state")
	}
	q.Set("state", state)
	if code != "" {
		q.Set("code", code)
	}
	cb.RawQuery = q.Encode()
	resp, err := http.Get(cb.String())
	if err != nil {
		return
	}
	resp.Body.Close()
}

func findProvider(cfg *config.Config, name string) (config.LLMConfig, bool) {
	for _, provider := range cfg.Providers {
		if provider.Provider == name {
			return provider, true
		}
	}
	return config.LLMConfig{}, false
}
