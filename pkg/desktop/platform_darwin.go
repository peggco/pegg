//go:build gui && darwin

package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func nativeTitlebar() bool {
	return envBool("PEGG_DESKTOP_NATIVE_TITLEBAR")
}

func applyPlatformOptions(opts *options.App, dark bool) {
	opts.Frameless = false
	appearance := mac.NSAppearanceNameAqua
	if dark {
		appearance = mac.NSAppearanceNameDarkAqua
	}
	opts.Mac = &mac.Options{
		TitleBar:   mac.TitleBarHiddenInset(),
		Appearance: appearance,
		About: &mac.AboutInfo{
			Title:   "Pegg",
			Message: "Pegg is a provider-agnostic AI coding agent.",
		},
		DisableEscapeExitsFullscreen: true,
	}
}

func reveal(path string) error {
	return exec.Command("open", "-R", path).Start()
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
