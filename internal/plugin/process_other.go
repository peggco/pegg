//go:build !linux

package plugin

import "os/exec"

func configurePluginCmd(*exec.Cmd) {}
