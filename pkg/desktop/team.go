//go:build gui

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/peggco/pegg/internal/connect"
	"github.com/peggco/pegg/internal/core/config"
)

const (
	connectLogLimit = 200
	teamLogLimit    = 200
)

type ConnectStatus struct {
	State     string   `json:"state"`
	PID       int      `json:"pid,omitempty"`
	StartedAt string   `json:"startedAt,omitempty"`
	Error     string   `json:"error,omitempty"`
	Logs      []string `json:"logs"`
}

type ConnectSettings struct {
	Enabled         bool          `json:"enabled"`
	RelayURL        string        `json:"relayUrl"`
	Token           string        `json:"token,omitempty"`
	ClearToken      bool          `json:"clearToken,omitempty"`
	TokenMasked     string        `json:"tokenMasked"`
	HasToken        bool          `json:"hasToken"`
	DeviceName      string        `json:"deviceName"`
	DeviceID        string        `json:"deviceId"`
	PublicKey       string        `json:"publicKey"`
	AutoApprove     bool          `json:"autoApprove"`
	MaxConcurrency  int           `json:"maxConcurrency"`
	ResponseTimeout int           `json:"responseTimeout"`
	AllowedMethods  []string      `json:"allowedMethods"`
	Status          ConnectStatus `json:"status"`
}

type TeamProvider struct {
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	APIKeyMasked string `json:"apiKeyMasked"`
}

type TeamStatus struct {
	State     string         `json:"state"`
	URL       string         `json:"url,omitempty"`
	Error     string         `json:"error,omitempty"`
	Logs      []string       `json:"logs"`
	Providers []TeamProvider `json:"providers"`
}

func cloneConnectStatus(in ConnectStatus) ConnectStatus {
	out := in
	out.Logs = append([]string(nil), in.Logs...)
	return out
}

func appendLog(logs []string, line string, limit int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return logs
	}
	logs = append(logs, line)
	if len(logs) > limit {
		logs = append([]string(nil), logs[len(logs)-limit:]...)
	}
	return logs
}

type logWriter struct {
	mu   sync.Mutex
	buf  strings.Builder
	emit func(string)
}

func newLogWriter(emit func(string)) *logWriter {
	return &logWriter{emit: emit}
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		s := w.buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(s[:idx], "\r")
		w.buf.Reset()
		w.buf.WriteString(s[idx+1:])
		if w.emit != nil {
			w.emit(line)
		}
	}
	return len(p), nil
}

func (a *App) connectConfig() (*config.ConnectConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	cc := cfg.Connect
	if cc == nil {
		cc = config.DefaultConfig().Connect
	}
	if strings.TrimSpace(cc.RelayURL) == "" {
		cc.RelayURL = config.DefaultConnectRelayURL
	}
	return cc, nil
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return s[:4] + "…" + s[len(s)-4:]
}

func isTeamProviderName(name string) bool {
	return name == "pegg-team" || strings.HasPrefix(name, "pegg-team ")
}

func (a *App) binaryPath() (string, error) {
	st := a.BinaryStatus()
	if !st.Found {
		return "", fmt.Errorf("pegg binary not found: %s", st.Error)
	}
	return st.Path, nil
}

func (a *App) ConnectSettings() (ConnectSettings, error) {
	cc, err := a.connectConfig()
	if err != nil {
		return ConnectSettings{}, err
	}
	a.connectMu.Lock()
	status := cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()

	out := ConnectSettings{
		Enabled:         cc.Enabled,
		RelayURL:        cc.RelayURL,
		TokenMasked:     connect.MaskToken(cc.Token),
		HasToken:        strings.TrimSpace(cc.Token) != "",
		DeviceName:      cc.DeviceName,
		AutoApprove:     cc.AutoApprove,
		MaxConcurrency:  cc.MaxConcurrency,
		ResponseTimeout: cc.ResponseTimeoutSecs,
		AllowedMethods:  append([]string(nil), cc.AllowedMethods...),
		Status:          status,
	}
	if id, err := connect.LoadOrCreateIdentity(cc.DeviceName); err == nil {
		out.DeviceID = id.DeviceID
		out.PublicKey = id.PublicKey
		if out.DeviceName == "" {
			out.DeviceName = id.Name
		}
	}
	if out.AllowedMethods == nil {
		out.AllowedMethods = []string{}
	}
	return out, nil
}

func (a *App) SaveConnectSettings(in ConnectSettings) (ConnectSettings, error) {
	cfg, err := config.Load()
	if err != nil {
		return ConnectSettings{}, err
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	cc := cfg.Connect
	if cc == nil {
		cc = config.DefaultConfig().Connect
	}
	cc.Enabled = in.Enabled
	if v := strings.TrimSpace(in.RelayURL); v != "" {
		cc.RelayURL = v
	}
	if in.ClearToken {
		cc.Token = ""
	} else if v := strings.TrimSpace(in.Token); v != "" {
		cc.Token = v
	}
	cc.DeviceName = strings.TrimSpace(in.DeviceName)
	cc.AutoApprove = in.AutoApprove
	if in.MaxConcurrency > 0 {
		cc.MaxConcurrency = in.MaxConcurrency
	}
	cc.ResponseTimeoutSecs = in.ResponseTimeout
	methods := make([]string, 0, len(in.AllowedMethods))
	for _, m := range in.AllowedMethods {
		if v := strings.TrimSpace(m); v != "" {
			methods = append(methods, v)
		}
	}
	cc.AllowedMethods = methods
	if strings.TrimSpace(cc.RelayURL) == "" {
		cc.RelayURL = config.DefaultConnectRelayURL
	}
	if err := config.UpsertConnect(cc); err != nil {
		return ConnectSettings{}, err
	}
	a.connectMu.Lock()
	running := a.connectCmd != nil
	a.connectMu.Unlock()
	if running {
		if !in.Enabled {
			_, _ = a.ConnectStop()
		} else {
			if _, err := a.ConnectStop(); err != nil {
				return ConnectSettings{}, err
			}
			if _, err := a.ConnectStart(); err != nil {
				return ConnectSettings{}, err
			}
		}
	}
	return a.ConnectSettings()
}

func (a *App) ConnectStart() (ConnectStatus, error) {
	a.connectMu.Lock()
	if a.connectCmd != nil {
		st := cloneConnectStatus(a.connectState)
		a.connectMu.Unlock()
		return st, nil
	}
	a.connectMu.Unlock()

	cc, err := a.connectConfig()
	if err != nil {
		return ConnectStatus{}, err
	}
	if strings.TrimSpace(cc.Token) == "" {
		return ConnectStatus{}, errors.New("connect: pairing token is required")
	}
	bin, err := a.binaryPath()
	if err != nil {
		return ConnectStatus{}, err
	}

	cmd := exec.Command(bin, "connect")
	cmd.Dir = a.currentWorkspace()
	cmd.Env = append(os.Environ(), "PEGG_CONNECT_LOG=console")
	hideWindow(cmd)

	a.connectMu.Lock()
	a.connectCmd = cmd
	a.connectStopping = false
	a.connectDone = make(chan struct{})
	a.connectState = ConnectStatus{
		State:     "starting",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Logs:      append([]string(nil), a.connectState.Logs...),
	}
	done := a.connectDone
	a.connectMu.Unlock()

	writer := newLogWriter(a.appendConnectLine)
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		a.connectMu.Lock()
		a.connectCmd = nil
		a.connectDone = nil
		a.connectState.State = "error"
		a.connectState.Error = err.Error()
		st := cloneConnectStatus(a.connectState)
		a.connectMu.Unlock()
		a.emit("connect:status", st)
		return st, err
	}

	a.connectMu.Lock()
	a.connectState.PID = cmd.Process.Pid
	st := cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()
	a.emit("connect:status", st)

	go a.waitConnect(cmd, done)
	return st, nil
}

func (a *App) waitConnect(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	a.connectMu.Lock()
	a.connectCmd = nil
	a.connectDone = nil
	stopping := a.connectStopping
	a.connectStopping = false
	switch {
	case stopping:
		a.connectState.State = "stopped"
		a.connectState.Error = ""
	case err != nil:
		a.connectState.State = "error"
		a.connectState.Error = err.Error()
	default:
		a.connectState.State = "stopped"
	}
	a.connectState.PID = 0
	st := cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()
	close(done)
	a.emit("connect:status", st)
}

func (a *App) appendConnectLine(line string) {
	a.connectMu.Lock()
	a.connectState.Logs = appendLog(a.connectState.Logs, line, connectLogLimit)
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "connected to relay"):
		a.connectState.State = "connected"
		a.connectState.Error = ""
	case strings.Contains(lower, "relay connection ended"):
		a.connectState.State = "reconnecting"
	case strings.Contains(lower, "refusing insecure"),
		strings.Contains(lower, "relay rejected"),
		strings.Contains(lower, "no pairing token"),
		strings.Contains(lower, "relay_url is not configured"):
		a.connectState.State = "error"
		a.connectState.Error = line
	}
	st := cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()
	a.emit("connect:log", line)
	a.emit("connect:status", st)
}

func (a *App) ConnectStop() (ConnectStatus, error) {
	a.connectMu.Lock()
	cmd := a.connectCmd
	done := a.connectDone
	if cmd == nil {
		st := cloneConnectStatus(a.connectState)
		a.connectMu.Unlock()
		return st, nil
	}
	a.connectStopping = true
	a.connectState.State = "stopping"
	st := cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()
	a.emit("connect:status", st)

	if cmd.Process != nil {
		_ = terminateProcess(cmd)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}
	a.connectMu.Lock()
	st = cloneConnectStatus(a.connectState)
	a.connectMu.Unlock()
	return st, nil
}

func (a *App) ConnectLogout() (ConnectSettings, error) {
	_, _ = a.ConnectStop()
	if err := config.Update(func(cfg *config.Config) error {
		if cfg.Connect == nil {
			cfg.Connect = config.DefaultConfig().Connect
		}
		cfg.Connect.Token = ""
		cfg.Connect.Enabled = false
		return nil
	}); err != nil {
		return ConnectSettings{}, err
	}
	return a.ConnectSettings()
}

func (a *App) ConnectResetDevice() (ConnectSettings, error) {
	_, _ = a.ConnectStop()
	if err := connect.ResetIdentity(); err != nil {
		return ConnectSettings{}, err
	}
	return a.ConnectSettings()
}

func (a *App) autoStartConnect() {
	cc, err := a.connectConfig()
	if err != nil || !cc.Enabled || strings.TrimSpace(cc.Token) == "" {
		return
	}
	if _, err := a.ConnectStart(); err != nil {
		a.logf("connect: auto-start failed: %v", err)
	}
}

func (a *App) teamProviders() []TeamProvider {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return []TeamProvider{}
	}
	out := make([]TeamProvider, 0)
	for _, p := range cfg.Providers {
		if !isTeamProviderName(p.Provider) {
			continue
		}
		out = append(out, TeamProvider{
			Name:         p.Provider,
			BaseURL:      p.BaseURL,
			APIKeyMasked: maskSecret(p.APIKey),
		})
	}
	return out
}

func (a *App) TeamStatus() TeamStatus {
	a.teamMu.Lock()
	st := a.teamState
	st.Logs = append([]string(nil), a.teamState.Logs...)
	a.teamMu.Unlock()
	st.Providers = a.teamProviders()
	if st.State == "" {
		st.State = "idle"
	}
	return st
}

func (a *App) TeamConnect() (TeamStatus, error) {
	a.teamMu.Lock()
	if a.teamCmd != nil {
		st := a.teamState
		st.Logs = append([]string(nil), a.teamState.Logs...)
		a.teamMu.Unlock()
		return st, nil
	}
	a.teamMu.Unlock()

	bin, err := a.binaryPath()
	if err != nil {
		return TeamStatus{}, err
	}
	cmd := exec.Command(bin, "team", "--no-browser")
	cmd.Dir = a.currentWorkspace()
	hideWindow(cmd)

	a.teamMu.Lock()
	a.teamCmd = cmd
	a.teamDone = make(chan struct{})
	a.teamState = TeamStatus{
		State: "starting",
		Logs:  append([]string(nil), a.teamState.Logs...),
	}
	done := a.teamDone
	a.teamMu.Unlock()

	writer := newLogWriter(a.appendTeamLine)
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		a.teamMu.Lock()
		a.teamCmd = nil
		a.teamDone = nil
		a.teamState.State = "error"
		a.teamState.Error = err.Error()
		st := a.teamState
		st.Logs = append([]string(nil), a.teamState.Logs...)
		a.teamMu.Unlock()
		a.emit("team:status", st)
		return st, err
	}
	st := a.TeamStatus()
	a.emit("team:status", st)
	go a.waitTeam(cmd, done)
	return st, nil
}

func (a *App) waitTeam(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	a.teamMu.Lock()
	a.teamCmd = nil
	a.teamDone = nil
	if err != nil {
		a.teamState.State = "error"
		a.teamState.Error = err.Error()
	} else {
		a.teamState.State = "done"
		a.teamState.Error = ""
	}
	st := a.teamState
	st.Logs = append([]string(nil), a.teamState.Logs...)
	a.teamMu.Unlock()
	st.Providers = a.teamProviders()
	close(done)
	a.emit("team:status", st)
}

func (a *App) appendTeamLine(line string) {
	a.teamMu.Lock()
	a.teamState.Logs = appendLog(a.teamState.Logs, line, teamLogLimit)
	lower := strings.ToLower(line)
	if u := strings.TrimSpace(line); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		a.teamState.URL = u
		a.teamState.State = "waiting"
	} else if strings.Contains(lower, "browser approval received") {
		a.teamState.State = "approving"
	} else if strings.Contains(lower, "team connection validated") {
		a.teamState.State = "validated"
	} else if strings.Contains(lower, "timed out") || strings.Contains(lower, "failed") || strings.Contains(lower, "cancelled") {
		a.teamState.Error = line
	}
	st := a.teamState
	st.Logs = append([]string(nil), a.teamState.Logs...)
	url := a.teamState.URL
	a.teamMu.Unlock()

	if url != "" && a.ctx != nil {
		wailsruntime.BrowserOpenURL(a.ctx, url)
		a.teamMu.Lock()
		a.teamState.URL = ""
		a.teamMu.Unlock()
	}
	a.emit("team:log", line)
	a.emit("team:status", st)
}

func (a *App) TeamCancel() (TeamStatus, error) {
	a.teamMu.Lock()
	cmd := a.teamCmd
	done := a.teamDone
	a.teamMu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = terminateProcess(cmd)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}
	return a.TeamStatus(), nil
}

func (a *App) TeamRefresh(provider string) (TeamStatus, error) {
	args := []string{"team", "--refresh"}
	if strings.TrimSpace(provider) != "" {
		args = append(args, "--provider", strings.TrimSpace(provider))
	}
	return a.runTeamCommand(args)
}

func (a *App) TeamRemove(provider string) (TeamStatus, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return TeamStatus{}, errors.New("team: provider name is required")
	}
	return a.runTeamCommand([]string{"team", "remove", provider})
}

func (a *App) runTeamCommand(args []string) (TeamStatus, error) {
	bin, err := a.binaryPath()
	if err != nil {
		return TeamStatus{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = a.currentWorkspace()
	hideWindow(cmd)
	writer := newLogWriter(func(line string) {
		a.teamMu.Lock()
		a.teamState.Logs = appendLog(a.teamState.Logs, line, teamLogLimit)
		a.teamMu.Unlock()
		a.emit("team:log", line)
	})
	cmd.Stdout = writer
	cmd.Stderr = writer
	a.teamMu.Lock()
	a.teamState.State = "running"
	a.teamMu.Unlock()
	a.emit("team:status", a.TeamStatus())
	err = cmd.Run()
	a.teamMu.Lock()
	if err != nil {
		a.teamState.State = "error"
		a.teamState.Error = err.Error()
	} else {
		a.teamState.State = "done"
		a.teamState.Error = ""
	}
	a.teamMu.Unlock()
	st := a.TeamStatus()
	a.emit("team:status", st)
	if err != nil {
		return st, err
	}
	return st, nil
}
