package cli

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
	"github.com/spf13/cobra"

	"github.com/peggco/pegg/internal/core/config"
)

var (
	teamVerifyAttempts = 8
	teamVerifyDelay    = 10 * time.Second
)

type teamOptions struct {
	team        string
	provider    string
	device      string
	keyName     string
	port        int
	noBrowser   bool
	print       bool
	printSecret bool
	force       bool
	refresh     bool
}

type teamProviderConfig struct {
	Name            string            `json:"name"`
	Driver          string            `json:"driver"`
	BaseURL         string            `json:"base_url"`
	APIKey          string            `json:"api_key"`
	Headers         map[string]string `json:"headers,omitempty"`
	Timeout         int               `json:"timeout"`
	SystemFirstOnly *bool             `json:"system_first_only,omitempty"`
}

type teamModelConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type teamAgentDefault struct {
	Model    string `json:"model,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

type teamRouterAgent struct {
	Default    []string            `json:"default,omitempty"`
	Difficulty map[string][]string `json:"difficulty,omitempty"`
	Images     []string            `json:"images,omitempty"`
}

type teamSmartRouter struct {
	Enabled       bool                       `json:"enabled"`
	DecisionModel string                     `json:"decision_model,omitempty"`
	Agents        map[string]teamRouterAgent `json:"agents,omitempty"`
}

type teamSTT struct {
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	StreamingModel string `json:"streaming_model,omitempty"`
}

type teamDefaults struct {
	Agents      map[string]teamAgentDefault `json:"agents,omitempty"`
	SmartRouter *teamSmartRouter            `json:"smart_router,omitempty"`
	STT         *teamSTT                    `json:"stt,omitempty"`
}

type teamConfig struct {
	Team struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	Provider teamProviderConfig `json:"provider"`
	Models   []teamModelConfig  `json:"models"`
	Defaults teamDefaults       `json:"defaults"`
	Version  int                `json:"version"`
}

type teamExchangeResponse struct {
	Config teamConfig `json:"config"`
}

type callbackResult struct {
	code string
	err  error
}

func (c *CLI) newTeamCommand() *cobra.Command {
	opts := &teamOptions{}
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Connect this computer to a team's BYOK configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.refresh {
				return c.runTeamRefresh(cmd.OutOrStdout(), opts)
			}
			return c.runTeamConnect(cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.team, "team", "", "team name to preselect on the website")
	cmd.Flags().StringVar(&opts.provider, "provider", "", "team provider to refresh or remove")
	cmd.Flags().StringVar(&opts.device, "device", "", "device name shown on the website")
	cmd.Flags().StringVar(&opts.keyName, "key-name", "", "name for the created API key")
	cmd.Flags().IntVar(&opts.port, "port", 0, "local callback port (0 chooses a free port)")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "print the URL instead of opening a browser")
	cmd.Flags().BoolVar(&opts.print, "print", false, "print the merged pegg config after connecting")
	cmd.Flags().BoolVar(&opts.printSecret, "print-secret", false, "include API keys in --print output")
	cmd.Flags().BoolVar(&opts.force, "force", false, "save the provider even if its /models check fails")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "refresh defaults from the team service without creating a new key")

	cmd.AddCommand(c.newTeamListCommand(), c.newTeamRemoveCommand())
	return cmd
}

func (c *CLI) runTeamConnect(out io.Writer, opts *teamOptions) error {
	device := strings.TrimSpace(opts.device)
	if device == "" {
		device, _ = os.Hostname()
	}
	if device == "" {
		device = "CLI"
	}

	state, err := randomURLToken(32)
	if err != nil {
		return fmt.Errorf("cli: generate state: %w", err)
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		return fmt.Errorf("cli: generate verifier: %w", err)
	}
	challenge := pkceChallenge(verifier)

	listenAddr := "127.0.0.1:0"
	if opts.port > 0 {
		listenAddr = fmt.Sprintf("127.0.0.1:%d", opts.port)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("cli: start local callback: %w", err)
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
			case resultCh <- callbackResult{err: fmt.Errorf("authorization was cancelled")}:
			default:
			}
			return
		}
		if r.URL.Query().Get("state") != state {
			writeCallbackPage(w, "Invalid state", "Start pegg team again.")
			select {
			case resultCh <- callbackResult{err: errors.New("callback state did not match")}:
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			writeCallbackPage(w, "Missing code", "Start pegg team again.")
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

	connectURL := buildTeamConnectURL(config.AppUrl, state, challenge, callback, device, opts)
	fmt.Fprintf(out, "Open this URL to connect this computer:\n%s\n\n", connectURL)
	if !opts.noBrowser {
		if err := openBrowser(connectURL); err != nil {
			fmt.Fprintf(out, "Could not open a browser: %v\n", err)
		}
	}

	waitSpinner := startTeamSpinner(out, "Waiting for browser approval")
	var result callbackResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Minute):
		waitSpinner.Stop()
		return errors.New("cli: timed out waiting for browser approval")
	}
	waitSpinner.Stop()
	if result.err != nil {
		return result.err
	}
	fmt.Fprintln(out, "Browser approval received.")

	cfg, err := exchangeTeamCode(config.AppApiUrl, result.code, verifier)
	if err != nil {
		return err
	}
	if cfg.Provider.APIKey == "" {
		return errors.New("cli: the exchange response did not include an API key")
	}
	validateSpinner := startTeamSpinner(out, "Fetching models from team service (this can take up to a minute)")
	verifyErr := verifyTeamProvider(cfg.Provider.BaseURL, cfg.Provider.APIKey)
	if verifyErr != nil && !opts.force {
		validateSpinner.Stop()
		return fmt.Errorf("cli: provider check failed: %w (use --force to save anyway)", verifyErr)
	}
	if verifyErr != nil {
		validateSpinner.Stop()
		fmt.Fprintf(out, "Warning: provider check failed: %v; saving anyway\n", verifyErr)
	} else {
		validateSpinner.Success("Team connection validated.")
	}

	if err := config.Update(func(peggCfg *config.Config) error {
		applyTeamConfig(peggCfg, cfg)
		return nil
	}); err != nil {
		return fmt.Errorf("cli: save team config: %w", err)
	}

	fmt.Fprintf(out, "Connected to %s\n", cfg.Team.Name)
	fmt.Fprintf(out, "provider: %s\n", cfg.Provider.Name)
	fmt.Fprintf(out, "base URL: %s\n", cfg.Provider.BaseURL)
	fmt.Fprintf(out, "models: %d\n", len(cfg.Models))
	if opts.print {
		return printResolvedConfig(out, opts.printSecret)
	}
	return nil
}

func (c *CLI) runTeamRefresh(out io.Writer, opts *teamOptions) error {
	peggCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cli: load config: %w", err)
	}
	provider, err := findTeamProvider(peggCfg, opts.provider)
	if err != nil {
		return err
	}
	cfg, err := fetchTeamConfig(provider.BaseURL, provider.APIKey)
	if err != nil {
		return err
	}
	cfg.Provider = teamProviderConfig{
		Name:    provider.Provider,
		Driver:  provider.Driver,
		BaseURL: provider.BaseURL,
		APIKey:  provider.APIKey,
		Headers: provider.Headers,
		Timeout: provider.Timeout,
	}
	if err := config.Update(func(peggCfg *config.Config) error {
		applyTeamConfig(peggCfg, cfg)
		return nil
	}); err != nil {
		return fmt.Errorf("cli: save team config: %w", err)
	}
	fmt.Fprintf(out, "Refreshed defaults for %s\n", provider.Provider)
	if opts.print {
		return printResolvedConfig(out, opts.printSecret)
	}
	return nil
}

func (c *CLI) newTeamListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connected team providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			peggCfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("cli: load config: %w", err)
			}
			found := false
			for _, p := range peggCfg.Providers {
				if !isTeamProvider(p.Provider) {
					continue
				}
				found = true
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-40s %s\n", p.Provider, p.BaseURL, maskAPIKey(p.APIKey))
			}
			if !found {
				fmt.Fprintln(cmd.OutOrStdout(), "no team providers configured")
			}
			return nil
		},
	}
}

func (c *CLI) newTeamRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <provider-name>",
		Short: "Remove a connected team provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !isTeamProvider(name) {
				return fmt.Errorf("cli: %q is not a team provider", name)
			}
			var removed *config.LLMConfig
			if err := config.Update(func(peggCfg *config.Config) error {
				providers := peggCfg.Providers[:0]
				for i := range peggCfg.Providers {
					if peggCfg.Providers[i].Provider == name {
						copyCfg := peggCfg.Providers[i]
						removed = &copyCfg
						continue
					}
					providers = append(providers, peggCfg.Providers[i])
				}
				peggCfg.Providers = providers
				if removed == nil {
					return fmt.Errorf("provider %q not found", name)
				}
				if peggCfg.STT != nil {
					kept := peggCfg.STT.Providers[:0]
					for _, entry := range peggCfg.STT.Providers {
						if entry.Provider == "pegg-team" && entry.BaseURL == removed.BaseURL {
							continue
						}
						kept = append(kept, entry)
					}
					peggCfg.STT.Providers = kept
					if peggCfg.STT.Provider == "pegg-team" {
						peggCfg.STT.Provider = ""
						peggCfg.STT.Model = ""
						peggCfg.STT.StreamingModel = ""
					}
				}
				if peggCfg.SmartRouter != nil && peggCfg.SmartRouter.Provider == name {
					peggCfg.SmartRouter.Provider = ""
				}
				for agent, def := range peggCfg.AgentDefaults {
					if def.Provider == name {
						delete(peggCfg.AgentDefaults, agent)
					}
				}
				return nil
			}); err != nil {
				return fmt.Errorf("cli: remove team provider: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", name)
			return nil
		},
	}
}

func applyTeamConfig(peggCfg *config.Config, cfg *teamConfig) {
	providerName := strings.TrimSpace(cfg.Provider.Name)
	if providerName == "" {
		providerName = "pegg-team"
		if cfg.Team.Name != "" {
			providerName += " " + cfg.Team.Name
		}
	}
	driver := firstNonEmpty(cfg.Provider.Driver, "openai")
	timeout := cfg.Provider.Timeout
	if timeout <= 0 {
		timeout = 300
	}
	provider := config.LLMConfig{
		Provider:        providerName,
		Driver:          driver,
		APIKey:          cfg.Provider.APIKey,
		BaseURL:         cfg.Provider.BaseURL,
		Headers:         cfg.Provider.Headers,
		Timeout:         timeout,
		SystemFirstOnly: cfg.Provider.SystemFirstOnly,
	}
	replaced := false
	for i := range peggCfg.Providers {
		if peggCfg.Providers[i].Provider == providerName {
			peggCfg.Providers[i] = provider
			replaced = true
			break
		}
	}
	if !replaced {
		peggCfg.Providers = append(peggCfg.Providers, provider)
	}

	if cfg.Defaults.Agents != nil {
		if peggCfg.AgentDefaults == nil {
			peggCfg.AgentDefaults = make(map[string]config.AgentModelDefault)
		}
		for agent, def := range cfg.Defaults.Agents {
			if def.Model == "" && def.Fallback == "" {
				continue
			}
			peggCfg.AgentDefaults[agent] = config.AgentModelDefault{
				Provider:         providerName,
				Model:            def.Model,
				FallbackProvider: providerName,
				FallbackModel:    def.Fallback,
			}
		}
	}

	if cfg.Defaults.SmartRouter != nil {
		router := cfg.Defaults.SmartRouter
		agents := make(map[string]config.RouterAgentConfig, len(router.Agents))
		for name, agent := range router.Agents {
			agents[name] = config.RouterAgentConfig{
				Default:    agent.Default,
				Difficulty: agent.Difficulty,
				Images:     agent.Images,
			}
		}
		peggCfg.SmartRouter = &config.SmartRouterConfig{
			Enabled:  router.Enabled,
			Provider: providerName,
			Model:    router.DecisionModel,
			Agents:   agents,
		}
	}

	if cfg.Defaults.STT != nil && cfg.Defaults.STT.Model != "" {
		if peggCfg.STT == nil {
			peggCfg.STT = &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
		}
		peggCfg.STT.Provider = "pegg-team"
		peggCfg.STT.Model = cfg.Defaults.STT.Model
		peggCfg.STT.StreamingModel = cfg.Defaults.STT.StreamingModel
		peggCfg.STT.UpsertProvider(config.STTProviderConfig{
			Provider: "pegg-team",
			BaseURL:  cfg.Provider.BaseURL,
			APIKey:   cfg.Provider.APIKey,
		})
	}
}

func exchangeTeamCode(apiURL, code, verifier string) (*teamConfig, error) {
	body, _ := json.Marshal(map[string]string{
		"code":          code,
		"code_verifier": verifier,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, teamExchangeURL(apiURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cli: exchange authorization: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cli: exchange failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out teamExchangeResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("cli: decode exchange response: %w", err)
	}
	return &out.Config, nil
}

func fetchTeamConfig(baseURL, apiKey string) (*teamConfig, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/team/config"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cli: fetch team config: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cli: team config failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var cfg teamConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cli: decode team config: %w", err)
	}
	return &cfg, nil
}

func verifyTeamProvider(baseURL, apiKey string) error {
	attempts := teamVerifyAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && teamVerifyDelay > 0 {
			time.Sleep(teamVerifyDelay)
		}
		if err := verifyTeamProviderOnce(baseURL, apiKey); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func verifyTeamProviderOnce(baseURL, apiKey string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/models"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("models endpoint returned HTTP %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("models endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type teamSpinner struct {
	w     io.Writer
	stop  chan struct{}
	done  chan struct{}
	label string
}

func startTeamSpinner(w io.Writer, label string) *teamSpinner {
	if w == nil {
		w = io.Discard
	}
	s := &teamSpinner{w: w, stop: make(chan struct{}), done: make(chan struct{}), label: label}
	if !isTerminalWriter(w) {
		fmt.Fprintf(w, "%s...\n", label)
		close(s.done)
		return s
	}
	go s.run()
	return s
}

func (s *teamSpinner) run() {
	defer close(s.done)
	frames := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Fprint(s.w, "\r\033[K")
			return
		case <-ticker.C:
			fmt.Fprintf(s.w, "\r%s %s...", frames[i%len(frames)], s.label)
			i++
		}
	}
}

func (s *teamSpinner) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *teamSpinner) Success(message string) {
	s.Stop()
	fmt.Fprintln(s.w, message)
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func buildTeamConnectURL(appURL, state, challenge, callback, device string, opts *teamOptions) string {
	base := strings.TrimRight(appURL, "/") + "/app/team-cli"
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("state", state)
	q.Set("challenge", challenge)
	q.Set("callback", callback)
	q.Set("device", device)
	if team := strings.TrimSpace(opts.team); team != "" {
		q.Set("team", team)
	}
	if keyName := strings.TrimSpace(opts.keyName); keyName != "" {
		q.Set("key_name", keyName)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func teamExchangeURL(apiURL string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if strings.HasSuffix(base, "/api") {
		return base + "/team-ai/cli/exchange"
	}
	return base + "/api/team-ai/cli/exchange"
}

func findTeamProvider(peggCfg *config.Config, hint string) (*config.LLMConfig, error) {
	var candidates []*config.LLMConfig
	for i := range peggCfg.Providers {
		if isTeamProvider(peggCfg.Providers[i].Provider) {
			candidates = append(candidates, &peggCfg.Providers[i])
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("cli: no team provider configured; run `pegg team` first")
	}
	hint = strings.TrimSpace(hint)
	if hint == "" {
		if len(candidates) > 1 {
			var names []string
			for _, p := range candidates {
				names = append(names, p.Provider)
			}
			return nil, fmt.Errorf("cli: multiple team providers configured (%s); pass --provider", strings.Join(names, ", "))
		}
		return candidates[0], nil
	}
	for _, p := range candidates {
		if p.Provider == hint || strings.EqualFold(strings.TrimPrefix(p.Provider, "pegg-team "), hint) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("cli: team provider %q not found", hint)
}

func isTeamProvider(name string) bool {
	return name == "pegg-team" || strings.HasPrefix(name, "pegg-team ")
}

func printResolvedConfig(out io.Writer, includeSecrets bool) error {
	peggCfg, err := config.Load()
	if err != nil {
		return err
	}
	masked := *peggCfg
	masked.Providers = append([]config.LLMConfig(nil), peggCfg.Providers...)
	for i := range masked.Providers {
		if !includeSecrets {
			masked.Providers[i].APIKey = maskAPIKey(masked.Providers[i].APIKey)
		}
	}
	if masked.STT != nil {
		sttCopy := *masked.STT
		sttCopy.Providers = append([]config.STTProviderConfig(nil), masked.STT.Providers...)
		for i := range sttCopy.Providers {
			if !includeSecrets {
				sttCopy.Providers[i].APIKey = maskAPIKey(sttCopy.Providers[i].APIKey)
			}
		}
		masked.STT = &sttCopy
	}
	data, err := json.MarshalIndent(masked, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func randomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
