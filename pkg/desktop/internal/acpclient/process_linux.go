//go:build linux

package acpclient

import (
	"os/exec"
	"syscall"
)

func setParentDeathSignal(cmd *exec.Cmd) {
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
}
