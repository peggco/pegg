//go:build !windows

package acpclient

import (
	"os/exec"
	"testing"
	"time"
)

func TestPrepareCommandSetsProcessGroup(t *testing.T) {
	cmd := exec.Command("true")
	prepareCommand(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr = %+v, want Setpgid", cmd.SysProcAttr)
	}
}

func TestForceKillNil(t *testing.T) {
	if err := forceKill(nil); err != nil {
		t.Fatalf("forceKill(nil) = %v", err)
	}
	if err := forceKill(&exec.Cmd{}); err != nil {
		t.Fatalf("forceKill(empty) = %v", err)
	}
}

func TestForceKillTerminatesProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")
	prepareCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sh: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	if err := forceKill(cmd); err != nil {
		t.Fatalf("forceKill = %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("process group did not exit after forceKill")
	}
}
