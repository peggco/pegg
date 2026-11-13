//go:build !windows

package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyUnix(t *testing.T) {
	manager, target, cache := newTestManager(t, "0.2.4")
	version := "0.2.5"
	stageDir := filepath.Join(cache, version)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(stageDir, BinaryName("linux"))
	if err := os.WriteFile(staged, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	manager.mu.Lock()
	manager.stagedPath = staged
	manager.stagedVersion = version
	manager.status.Supported = true
	manager.mu.Unlock()

	if err := manager.ApplyDesktop(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("target = %q", data)
	}
	if _, err := os.Stat(target + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new should be removed")
	}
}
