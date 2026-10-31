//go:build gui && linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func nativeTitlebar() bool {
	if envBool("PEGG_DESKTOP_FRAMELESS") {
		return false
	}
	if envBool("PEGG_DESKTOP_NATIVE_TITLEBAR") {
		return true
	}
	return true
}

func applyPlatformOptions(opts *options.App, dark bool) {
	opts.Frameless = !nativeTitlebar()
}

func reveal(path string) error {
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		dir = path
	}
	return exec.Command("xdg-open", dir).Start()
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
