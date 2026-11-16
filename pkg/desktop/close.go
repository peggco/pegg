//go:build gui

package main

import (
	"context"
	"os"
	"time"

	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
	"github.com/peggco/pegg/pkg/desktop/internal/closer"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	shutdownDeadline        = 2 * time.Second
	closingOverlayThreshold = 300 * time.Millisecond
)

func (a *App) takeClient() *acpclient.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.client
	a.client = nil
	return c
}

func (a *App) runShutdown() {
	a.mu.Lock()
	a.closing = true
	if a.viewCancel != nil {
		a.viewCancel()
	}
	a.mu.Unlock()

	c := a.takeClient()
	outcome := closer.Run(a.shutdownTasks(c), closer.Options{
		Deadline:  shutdownDeadline,
		Threshold: closingOverlayThreshold,
		Logf:      a.logf,
		OnSlow: func() {
			a.emit("app:closing", true)
		},
	})
	if outcome.TimedOut {
		a.logf("shutdown: forcing exit after %s", outcome.Duration)
		if c != nil {
			c.Kill()
		}
		a.killChildProcesses()
		os.Exit(0)
	}
	a.closeMu.Lock()
	a.closeReady = true
	a.closeMu.Unlock()
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
		return
	}
	os.Exit(0)
}

func (a *App) shutdownTasks(c *acpclient.Client) []closer.Task {
	tasks := []closer.Task{
		{
			Name: "plugins",
			Run: func(context.Context) error {
				a.resetPluginManager()
				return nil
			},
		},
		{
			Name: "connect",
			Run: func(context.Context) error {
				_, err := a.ConnectStop()
				return err
			},
		},
		{
			Name: "team",
			Run: func(context.Context) error {
				_, err := a.TeamCancel()
				return err
			},
		},
		{
			Name: "pegg",
			Run: func(context.Context) error {
				_, err := a.PeggCancel()
				return err
			},
		},
	}
	if c != nil {
		tasks = append(tasks, closer.Task{
			Name: "acp",
			Run: func(ctx context.Context) error {
				c.CloseWithContext(ctx)
				return nil
			},
		})
	}
	return tasks
}

func (a *App) killChildProcesses() {
	a.connectMu.Lock()
	connectCmd := a.connectCmd
	a.connectMu.Unlock()
	_ = forceKillProcess(connectCmd)

	a.teamMu.Lock()
	teamCmd := a.teamCmd
	a.teamMu.Unlock()
	_ = forceKillProcess(teamCmd)

	a.peggMu.Lock()
	peggCmd := a.peggCmd
	a.peggMu.Unlock()
	_ = forceKillProcess(peggCmd)
}
