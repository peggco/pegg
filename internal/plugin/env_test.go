package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

func TestMergePluginEnvPrecedence(t *testing.T) {
	base := []string{"A=1", "PATH=/bin", "EMPTY="}
	got := mergePluginEnv(base, map[string]string{"A": "2", "B": "3"})
	want := []string{"A=2", "B=3", "EMPTY=", "PATH=/bin"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("merged env = %v, want %v", got, want)
	}
}

func TestMergePluginEnvWithoutOverridesKeepsBase(t *testing.T) {
	base := []string{"A=1"}
	got := mergePluginEnv(base, nil)
	if len(got) != 1 || got[0] != "A=1" {
		t.Fatalf("merged env = %v", got)
	}
}

func TestMaskEnv(t *testing.T) {
	masked := MaskEnv(map[string]string{
		"TOKEN": "super-secret-token",
		"SHORT": "abc",
		"EMPTY": "",
	})
	if masked["TOKEN"] == "super-secret-token" || !strings.Contains(masked["TOKEN"], "…") {
		t.Fatalf("long value not masked: %q", masked["TOKEN"])
	}
	if masked["SHORT"] != "••••" || masked["EMPTY"] != "" {
		t.Fatalf("masked values: %#v", masked)
	}
	joined := strings.Join([]string{masked["TOKEN"], masked["SHORT"]}, "")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "abc") {
		t.Fatalf("mask leaked a secret: %q", joined)
	}
}

func TestPluginEnvResolvesAliases(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Plugins.Env = map[string]map[string]string{
		"Kanban GitHub": {"PEGG_GITHUB_TOKEN": "token"},
	}
	mgr := NewManager(logger.New(logger.LevelError), cfg, nil, nil, nil, nil, nil, nil, nil)
	if got := mgr.pluginEnv("pegg-kanban-github"); got["PEGG_GITHUB_TOKEN"] != "token" {
		t.Fatalf("env not resolved: %#v", got)
	}
	if got := mgr.pluginEnv("unrelated"); got != nil {
		t.Fatalf("unrelated plugin env resolved: %#v", got)
	}
}

func TestPluginCmdInjectsEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell script")
	}
	t.Setenv("PEGG_INHERITED", "inherited")
	t.Setenv("PEGG_TEST_TOKEN", "parent")
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	script := filepath.Join(dir, "capture-env")
	body := "#!/bin/sh\nprintf '%s|%s' \"$PEGG_TEST_TOKEN\" \"$PEGG_INHERITED\" > " + out + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := pluginCmd(script, map[string]string{"PEGG_TEST_TOKEN": "configured"})
	if err := cmd.Run(); err != nil {
		t.Fatalf("run plugin cmd: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "configured|inherited" {
		t.Fatalf("plugin env = %q, want configured|inherited", got)
	}
}
