//go:build !windows

package binary

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
