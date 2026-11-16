//go:build unix && !linux

package acpclient

import "os/exec"

func setParentDeathSignal(*exec.Cmd) {}
