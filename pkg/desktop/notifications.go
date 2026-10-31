//go:build gui

package main

import "github.com/gen2brain/beeep"

func (a *App) Notify(title, body string) {
	if title == "" {
		title = "Pegg"
	}
	beeep.AppName = "Pegg"
	go func() {
		_ = beeep.Notify(title, body, "")
	}()
}
