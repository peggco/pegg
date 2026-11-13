package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newTestManager(t *testing.T, current string) (*Manager, string, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "pegg-desktop")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	return NewManager(current, target, cache), target, cache
}

func TestNewManagerSupported(t *testing.T) {
	manager, _, _ := newTestManager(t, "0.2.4")
	status := manager.Status()
	if status.CurrentVersion != "0.2.4" {
		t.Fatalf("current = %q", status.CurrentVersion)
	}
	wantSupported := runtime.GOOS != "darwin"
	if status.Supported != wantSupported {
		t.Fatalf("supported = %v, want %v", status.Supported, wantSupported)
	}
}

func TestLoadStagedRestoresReady(t *testing.T) {
	_, target, cache := newTestManager(t, "0.2.4")
	version := "0.2.5"
	stageDir := filepath.Join(cache, version)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(stageDir, BinaryName("linux"))
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest{
		Version: version,
		Asset:   "pegg-desktop_0.2.5_linux_amd64.tar.gz",
		Path:    staged,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, manifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager("0.2.4", target, cache)
	status := manager.Status()
	if !status.Ready || !status.Available || status.State != StateReady {
		t.Fatalf("status = %#v", status)
	}
}

func TestLoadStagedRestoresCombinedReady(t *testing.T) {
	_, target, cache := newTestManager(t, "0.2.4")
	version := "0.2.5"
	stageDir := filepath.Join(cache, version)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(stageDir, "pegg-desktop")
	agent := filepath.Join(stageDir, "pegg")
	if err := os.WriteFile(desktop, []byte("new-desktop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent, []byte("new-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	desktopManifest, _ := json.Marshal(manifest{Version: version, Path: desktop})
	agentManifest, _ := json.Marshal(manifest{Version: version, Path: agent})
	if err := os.WriteFile(filepath.Join(cache, manifestName), desktopManifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, agentManifestName), agentManifest, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager("0.2.4", target, cache)
	manager.SetAgent("0.2.4", true, filepath.Join(target+".agent"))
	status := manager.Status()
	if !status.DesktopReady || !status.AgentReady || !status.Ready || status.State != StateReady {
		t.Fatalf("status = %#v", status)
	}
}

func TestLoadStagedDropsOldVersion(t *testing.T) {
	_, target, cache := newTestManager(t, "0.2.4")
	version := "0.2.3"
	stageDir := filepath.Join(cache, version)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(stageDir, BinaryName("linux"))
	if err := os.WriteFile(staged, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest{Version: version, Path: staged})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, manifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager("0.2.4", target, cache)
	if manager.Status().Ready {
		t.Fatalf("old staged update should be dropped")
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged file should be removed")
	}
}

func TestCleanupRemovesOldFiles(t *testing.T) {
	manager, target, cache := newTestManager(t, "0.2.4")
	if err := os.WriteFile(target+".old", []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".new", []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(cache, "0.2.0")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}

	manager.Cleanup()
	for _, path := range []string{target + ".old", target + ".new", stale} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed", path)
		}
	}
}
