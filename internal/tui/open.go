package tui

import (
	"os/exec"
	"runtime"

	"github.com/peggco/pegg/internal/tui/i18n"
)

func (a *App) openChatFile(path string) {
	if path == "" {
		return
	}
	if err := openFile(path); err != nil {
		a.showError(i18n.Tf("app.error.openFile", err.Error()))
	}
}

func openFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
