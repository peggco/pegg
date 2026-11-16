//go:build unix

package acpclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCloseContextKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	script := filepath.Join(dir, "fake-pegg.sh")
	body := "#!/bin/sh\ntrap '' TERM\nsleep 60 &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	client := New(Options{Binary: script, Cwd: dir})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	grandchild := waitForPID(t, pidFile)
	t.Cleanup(func() {
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
	})
	if !processAlive(grandchild) {
		t.Fatalf("grandchild %d is not running", grandchild)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	start := time.Now()
	client.CloseContext(ctx)
	elapsed := time.Since(start)

	if elapsed > 1500*time.Millisecond {
		t.Fatalf("CloseContext took %s, want a bounded close", elapsed)
	}
	deadline := time.Now().Add(time.Second)
	for processAlive(grandchild) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processAlive(grandchild) {
		t.Fatalf("grandchild %d survived process-group close", grandchild)
	}
}

func waitForPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child pid file %s was not written", path)
	return 0
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
