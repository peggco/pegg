//go:build gui

package main

import (
	"context"

	"github.com/gen2brain/beeep"

	corehook "github.com/peggco/pegg/internal/core/hook"
)

func (a *App) Notify(title, body string) {
	if title == "" {
		title = "Pegg"
	}
	if hooked, errs := corehook.ApplyFilter(context.Background(), HookNotify, NotifyHook{Title: title, Body: body}); len(errs) == 0 {
		title, body = hooked.Title, hooked.Body
	}
	beeep.AppName = "Pegg"
	go func() {
		_ = beeep.Notify(title, body, "")
	}()
}
