//go:build gui

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/peggco/pegg/pkg/desktop/internal/binary"
	"github.com/peggco/pegg/pkg/desktop/internal/selfupdate"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) initSelfUpdate() {
	exe, err := os.Executable()
	if err != nil {
		a.logf("self-update: executable: %v", err)
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	manager := selfupdate.NewManager(appVersion, exe, filepath.Join(cache, "pegg", "updates"))
	manager.SetAgent(a.agentUpdateInfo())
	manager.Cleanup()
	a.mu.Lock()
	a.selfUpdate = manager
	a.mu.Unlock()
}

func (a *App) updateManager() *selfupdate.Manager {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.selfUpdate
}

func (a *App) agentUpdateInfo() (string, bool, string) {
	a.mu.Lock()
	state := a.binary
	initRes := a.initRes
	a.mu.Unlock()

	version := state.Version
	if version == "" && initRes != nil && initRes.AgentInfo != nil {
		version = initRes.AgentInfo.Version
	}
	target := ""
	if state.Found && state.Path != "" {
		target = state.Path
	} else {
		target = filepath.Join(binary.ManagedDir(), binary.Name())
	}
	return version, selfupdate.SupportedTarget(runtime.GOOS, filepath.Dir(target)), target
}

func (a *App) selfUpdateStatus() selfupdate.Status {
	manager := a.updateManager()
	if manager == nil {
		return selfupdate.Status{CurrentVersion: appVersion, State: selfupdate.StateIdle}
	}
	manager.SetAgent(a.agentUpdateInfo())
	return manager.Status()
}

func (a *App) SelfUpdateStatus() selfupdate.Status {
	return a.selfUpdateStatus()
}

func (a *App) CheckDesktopUpdate() (selfupdate.Status, error) {
	manager := a.updateManager()
	if manager == nil {
		return selfupdate.Status{}, errors.New("self-update is unavailable")
	}
	manager.SetAgent(a.agentUpdateInfo())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := manager.Check(ctx)
	a.emit("desktop:update", status)
	return status, err
}

func (a *App) DownloadDesktopUpdate() (selfupdate.Status, error) {
	manager := a.updateManager()
	if manager == nil {
		return selfupdate.Status{}, errors.New("self-update is unavailable")
	}
	manager.SetAgent(a.agentUpdateInfo())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	status, err := manager.Download(ctx, func(update selfupdate.Status) {
		a.emit("desktop:update", update)
	})
	a.emit("desktop:update", status)
	return status, err
}

func (a *App) RestartToUpdate() error {
	manager := a.updateManager()
	if manager == nil {
		return errors.New("self-update is unavailable")
	}
	manager.SetAgent(a.agentUpdateInfo())
	status := manager.Status()
	applyDesktop := status.DesktopAvailable && status.Supported && status.DesktopReady
	applyAgent := status.AgentAvailable && status.AgentSupported && status.AgentReady
	if !applyDesktop && !applyAgent {
		return errors.New("no update is ready to install")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if applyDesktop {
		if err := manager.ApplyDesktop(ctx); err != nil {
			return err
		}
	}
	if applyAgent {
		a.mu.Lock()
		client := a.client
		a.client = nil
		a.mu.Unlock()
		if client != nil {
			client.Close()
		}
		if err := manager.ApplyAgent(ctx); err != nil {
			return err
		}
	}

	if err := selfupdate.Relaunch(manager.Target()); err != nil {
		return err
	}
	wailsruntime.Quit(a.ctx)
	return nil
}

func (a *App) autoCheckDesktopUpdate() {
	time.Sleep(3 * time.Second)
	manager := a.updateManager()
	if manager == nil {
		return
	}
	manager.SetAgent(a.agentUpdateInfo())
	status, err := a.CheckDesktopUpdate()
	if err != nil || !status.Available {
		return
	}
	desktopNeeded := status.DesktopAvailable && status.Supported && !status.DesktopReady
	agentNeeded := status.AgentAvailable && status.AgentSupported && !status.AgentReady
	if !desktopNeeded && !agentNeeded {
		return
	}
	if _, err := a.DownloadDesktopUpdate(); err != nil {
		a.logf("self-update: download: %v", err)
	}
}
