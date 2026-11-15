package pegglogin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

const (
	ProviderName   = "pegg-forever"
	DefaultBaseURL = "https://forever.pegg.dev/v1"
	DefaultTimeout = 300

	defaultDriver       = "openai"
	browserWaitTimeout  = 5 * time.Minute
	exchangeTimeout     = 60 * time.Second
	verificationTimeout = 10 * time.Second
)

var (
	appURL    = config.AppUrl
	appAPIURL = config.AppApiUrl
)

type Config struct {
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Timeout int    `json:"timeout"`
}

type Options struct {
	Device      string
	Port        int
	NoBrowser   bool
	OnURL       func(string)
	OpenBrowser func(string) error
	HTTPClient  *http.Client
	Timeout     time.Duration
}

type exchangeResponse struct {
	Config struct {
		Provider Config `json:"provider"`
	} `json:"config"`
}

type callbackResult struct {
	code string
	err  error
}

func Login(ctx context.Context, opts Options) (*Config, error) {
	device := strings.TrimSpace(opts.Device)
	if device == "" {
		device, _ = os.Hostname()
	}
	if device == "" {
		device = "CLI"
	}

	state, err := randomToken(32)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: generate state: %w", err)
	}
	verifier, err := randomToken(48)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: generate verifier: %w", err)
	}
	challenge := PKCEChallenge(verifier)

	listenAddr := "127.0.0.1:0"
	if opts.Port > 0 {
		listenAddr = fmt.Sprintf("127.0.0.1:%d", opts.Port)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: start local callback: %w", err)
	}
	defer listener.Close()

	callback := fmt.Sprintf("http://%s/callback", listener.Addr().String())
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			writeCallbackPage(w, "Connection cancelled", "You can close this window.")
			select {
			case resultCh <- callbackResult{err: errors.New("authorization was cancelled")}:
			default:
			}
			return
		}
		if r.URL.Query().Get("state") != state {
			writeCallbackPage(w, "Invalid state", "Start pegg login again.")
			select {
			case resultCh <- callbackResult{err: errors.New("callback state did not match")}:
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			writeCallbackPage(w, "Missing code", "Start pegg login again.")
			select {
			case resultCh <- callbackResult{err: errors.New("callback did not include a code")}:
			default:
			}
			return
		}
		writeCallbackPage(w, "Connected", "You can close this window and return to pegg.")
		select {
		case resultCh <- callbackResult{code: code}:
		default:
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())

	authorizeURL := BuildAuthorizeURL(appURL, state, challenge, callback, device)
	if opts.OnURL != nil {
		opts.OnURL(authorizeURL)
	}
	if !opts.NoBrowser {
		opener := opts.OpenBrowser
		if opener == nil {
			opener = openBrowser
		}
		_ = opener(authorizeURL)
	}

	wait := opts.Timeout
	if wait <= 0 {
		wait = browserWaitTimeout
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()

	var result callbackResult
	select {
	case result = <-resultCh:
	case <-timer.C:
		return nil, errors.New("pegglogin: timed out waiting for browser approval")
	case <-ctx.Done():
		return nil, fmt.Errorf("pegglogin: wait for browser approval: %w", ctx.Err())
	}
	if result.err != nil {
		return nil, result.err
	}

	cfg, err := Exchange(ctx, "", result.code, verifier, opts.HTTPClient)
	if err != nil {
		return nil, err
	}
	if cfg.APIKey == "" {
		return nil, errors.New("pegglogin: the exchange response did not include an API key")
	}
	return cfg, nil
}

func Exchange(ctx context.Context, apiURL, code, verifier string, client *http.Client) (*Config, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("pegglogin: exchange requires a code")
	}
	body, err := json.Marshal(map[string]string{
		"code":          code,
		"code_verifier": verifier,
	})
	if err != nil {
		return nil, fmt.Errorf("pegglogin: encode exchange request: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, exchangeEndpoint(apiURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("pegglogin: build exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pegglogin: exchange authorization: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pegglogin: exchange failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out exchangeResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("pegglogin: decode exchange response: %w", err)
	}
	cfg := normalizeConfig(out.Config.Provider)
	return &cfg, nil
}

func BuildAuthorizeURL(appURL, state, challenge, callback, device string) string {
	base := strings.TrimRight(strings.TrimSpace(appURL), "/") + "/app/pegg-forever"
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("state", state)
	q.Set("challenge", challenge)
	q.Set("callback", callback)
	q.Set("device", device)
	u.RawQuery = q.Encode()
	return u.String()
}

func Apply(cfg *Config) error {
	if cfg == nil {
		return errors.New("pegglogin: config is required")
	}
	normalized := normalizeConfig(*cfg)
	return config.Update(func(peggCfg *config.Config) error {
		replaced := false
		for i := range peggCfg.Providers {
			if peggCfg.Providers[i].Provider != ProviderName {
				continue
			}
			existing := peggCfg.Providers[i]
			existing.Driver = normalized.Driver
			existing.BaseURL = normalized.BaseURL
			if normalized.APIKey != "" {
				existing.APIKey = normalized.APIKey
			}
			existing.Timeout = normalized.Timeout
			peggCfg.Providers[i] = existing
			replaced = true
			break
		}
		if !replaced {
			peggCfg.Providers = append(peggCfg.Providers, config.LLMConfig{
				Provider: ProviderName,
				Driver:   normalized.Driver,
				BaseURL:  normalized.BaseURL,
				APIKey:   normalized.APIKey,
				Timeout:  normalized.Timeout,
			})
		}
		upsertSTTProvider(peggCfg, normalized)
		return nil
	})
}

func upsertSTTProvider(peggCfg *config.Config, cfg Config) {
	if peggCfg.STT == nil {
		peggCfg.STT = &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
	}
	entry, _ := peggCfg.STT.ProviderConfig(ProviderName)
	entry.Provider = ProviderName
	entry.Driver = cfg.Driver
	entry.BaseURL = cfg.BaseURL
	if cfg.APIKey != "" {
		entry.APIKey = cfg.APIKey
	}
	peggCfg.STT.UpsertProvider(entry)
}

func Verify(ctx context.Context, baseURL, apiKey string, client *http.Client) error {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = DefaultBaseURL
	}
	endpoint := strings.TrimRight(base, "/") + "/models"
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("pegglogin: build models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pegglogin: fetch models: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("pegglogin: models endpoint returned HTTP %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("pegglogin: models endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func normalizeConfig(cfg Config) Config {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		cfg.Name = ProviderName
	}
	cfg.Driver = strings.TrimSpace(cfg.Driver)
	if cfg.Driver == "" {
		cfg.Driver = defaultDriver
	}
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	return cfg
}

func exchangeEndpoint(apiURL string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if base == "" {
		base = strings.TrimRight(appAPIURL, "/")
	}
	if strings.HasSuffix(base, "/api") {
		return base + "/pegg-forever/cli/exchange"
	}
	return base + "/api/pegg-forever/cli/exchange"
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		if err := exec.Command("xdg-open", rawURL).Start(); err == nil {
			return nil
		}
		cmd = exec.Command("gio", "open", rawURL)
	}
	return cmd.Start()
}

func writeCallbackPage(w io.Writer, title, message string) {
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title></head><body style="font-family: system-ui, sans-serif; padding: 3rem; text-align: center;"><h1>%s</h1><p>%s</p></body></html>`, title, title, message)
}
