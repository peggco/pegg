//go:build windows

package hooks

import "os/exec"

func configureProcess(*exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
