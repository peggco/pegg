//go:build linux

package plugin

import (
	"os/exec"
	"syscall"
)

func configurePluginCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
