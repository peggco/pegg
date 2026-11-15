//go:build gui

package main

import (
	"os/exec"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/peggco/pegg/internal/core/config"
)

const (
	peggLogLimit          = 200
	peggForeverProviderID = "pegg-forever"
)

type ForeverStatus struct {
	State      string   `json:"state"`
	URL        string   `json:"url,omitempty"`
	Error      string   `json:"error,omitempty"`
	Logs       []string `json:"logs"`
	Configured bool     `json:"configured"`
	Provider   string   `json:"provider"`
}

type ProviderSetupInfo struct {
	Configured bool `json:"configured"`
	Dismissed  bool `json:"dismissed"`
}

type foreverLogUpdate struct {
	state string
	url   string
	err   string
}

func cloneForeverStatus(in ForeverStatus) ForeverStatus {
	out := in
	out.Logs = append([]string(nil), in.Logs...)
	return out
}

func isPeggForeverProviderName(name string) bool {
	return strings.TrimSpace(name) == peggForeverProviderID
}

func findPeggForeverProvider(cfg *config.Config) (config.LLMConfig, bool) {
	if cfg == nil {
		return config.LLMConfig{}, false
	}
	for _, p := range cfg.Providers {
		if isPeggForeverProviderName(p.Provider) {
			return p, true
		}
	}
	return config.LLMConfig{}, false
}

func parseForeverLogLine(line string) foreverLogUpdate {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		return foreverLogUpdate{state: "waiting", url: trimmed}
	case strings.Contains(lower, "browser approval received"):
		return foreverLogUpdate{state: "approving"}
	case strings.Contains(lower, "logged in to pegg"):
		return foreverLogUpdate{state: "done"}
	case strings.Contains(lower, "timed out"),
		strings.Contains(lower, "failed"),
		strings.Contains(lower, "cancelled"):
		return foreverLogUpdate{state: "error", err: trimmed}
	}
	return foreverLogUpdate{}
}

func (a *App) ProviderSetup() (ProviderSetupInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return ProviderSetupInfo{}, err
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	info := ProviderSetupInfo{Configured: config.HasConfiguredProvider(cfg)}
	if cfg.Onboarding != nil {
		info.Dismissed = cfg.Onboarding.ProviderSetupDismissed
	}
	return info, nil
}

func (a *App) DismissProviderSetup() error {
	return config.DismissProviderSetup()
}

func (a *App) PeggStatus() ForeverStatus {
	a.peggMu.Lock()
	st := cloneForeverStatus(a.peggState)
	a.peggMu.Unlock()
	if st.State == "" {
		st.State = "idle"
	}
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return st
	}
	if p, ok := findPeggForeverProvider(cfg); ok {
		st.Provider = p.Provider
		st.Configured = strings.TrimSpace(p.APIKey) != ""
	}
	return st
}

func (a *App) PeggLogin() (ForeverStatus, error) {
	a.peggMu.Lock()
	if a.peggCmd != nil {
		a.peggMu.Unlock()
		return a.PeggStatus(), nil
	}
	a.peggMu.Unlock()

	bin, err := a.binaryPath()
	if err != nil {
		return ForeverStatus{}, err
	}
	cmd := exec.Command(bin, "login-pegg", "--no-browser", "--no-defaults")
	cmd.Dir = a.currentWorkspace()
	hideWindow(cmd)

	a.peggMu.Lock()
	a.peggCmd = cmd
	a.peggDone = make(chan struct{})
	a.peggState = ForeverStatus{
		State: "starting",
		Logs:  append([]string(nil), a.peggState.Logs...),
	}
	done := a.peggDone
	a.peggMu.Unlock()

	writer := newLogWriter(a.appendPeggLine)
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		a.peggMu.Lock()
		a.peggCmd = nil
		a.peggDone = nil
		a.peggState.State = "error"
		a.peggState.Error = err.Error()
		st := cloneForeverStatus(a.peggState)
		a.peggMu.Unlock()
		a.emit("pegg:status", st)
		return st, err
	}

	st := a.PeggStatus()
	a.emit("pegg:status", st)
	go a.waitPegg(cmd, done)
	return st, nil
}

func (a *App) waitPegg(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	a.peggMu.Lock()
	a.peggCmd = nil
	a.peggDone = nil
	if err != nil {
		a.peggState.State = "error"
		a.peggState.Error = err.Error()
	} else {
		a.peggState.State = "done"
		a.peggState.Error = ""
	}
	a.peggMu.Unlock()
	st := a.PeggStatus()
	close(done)
	a.emit("pegg:status", st)
}

func (a *App) appendPeggLine(line string) {
	update := parseForeverLogLine(line)
	a.peggMu.Lock()
	a.peggState.Logs = appendLog(a.peggState.Logs, line, peggLogLimit)
	if update.state != "" {
		a.peggState.State = update.state
		if update.state == "error" {
			a.peggState.Error = update.err
		}
	}
	openURL := ""
	if update.url != "" {
		if a.peggState.URL != update.url {
			openURL = update.url
		}
		a.peggState.URL = update.url
	}
	st := cloneForeverStatus(a.peggState)
	a.peggMu.Unlock()

	if openURL != "" && a.ctx != nil {
		wailsruntime.BrowserOpenURL(a.ctx, openURL)
	}
	a.emit("pegg:log", line)
	a.emit("pegg:status", st)
}

func (a *App) PeggCancel() (ForeverStatus, error) {
	a.peggMu.Lock()
	cmd := a.peggCmd
	done := a.peggDone
	a.peggMu.Unlock()
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
	return a.PeggStatus(), nil
}

func (a *App) PeggLogout() (ForeverStatus, error) {
	if _, ok := config.GetProvider(peggForeverProviderID); ok {
		if err := config.RemoveProvider(peggForeverProviderID); err != nil {
			return a.PeggStatus(), err
		}
	}
	if err := config.Update(func(cfg *config.Config) error {
		if cfg.STT == nil {
			return nil
		}
		kept := cfg.STT.Providers[:0]
		for _, entry := range cfg.STT.Providers {
			if entry.Provider != peggForeverProviderID {
				kept = append(kept, entry)
			}
		}
		cfg.STT.Providers = kept
		if cfg.STT.Provider == peggForeverProviderID {
			cfg.STT.Provider = ""
			cfg.STT.Model = ""
			cfg.STT.StreamingModel = ""
		}
		return nil
	}); err != nil {
		return a.PeggStatus(), err
	}
	a.peggMu.Lock()
	a.peggState = ForeverStatus{
		State: "idle",
		Logs:  append([]string(nil), a.peggState.Logs...),
	}
	a.peggMu.Unlock()
	st := a.PeggStatus()
	a.emit("pegg:status", st)
	return st, nil
}
