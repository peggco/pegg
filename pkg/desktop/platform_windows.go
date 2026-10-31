//go:build gui && windows

package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func nativeTitlebar() bool {
	return envBool("PEGG_DESKTOP_NATIVE_TITLEBAR")
}

func applyPlatformOptions(opts *options.App, dark bool) {
	opts.Frameless = !nativeTitlebar()
	theme := windows.SystemDefault
	if dark {
		theme = windows.Dark
	}
	opts.Windows = &windows.Options{
		Theme:                theme,
		WebviewIsTransparent: false,
		WindowIsTranslucent:  false,
	}
}

func reveal(path string) error {
	return exec.Command("explorer", "/select,", path).Start()
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
