//go:build windows

package acpclient

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

func terminateProcessGroup(cmd *exec.Cmd) error {
	return killProcessGroup(cmd)
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
