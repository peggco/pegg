package env

import (
	"context"
	"os"
	"strings"
	"testing"
)

func stubShell(t *testing.T, values map[string]string) {
	t.Helper()
	ResetCache()
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("PEGG_SHELL_ENV", "")
	orig := shellEnvFunc
	shellEnvFunc = func(context.Context) (map[string]string, error) {
		return values, nil
	}
	t.Cleanup(func() {
		shellEnvFunc = orig
		ResetCache()
	})
}

func TestAugmentMergesPathAndTokens(t *testing.T) {
	stubShell(t, map[string]string{
		"PATH":         "/opt/homebrew/bin:/usr/bin",
		"GH_TOKEN":     "from-shell",
		"GITHUB_TOKEN": "shell-github",
	})

	base := []string{"PATH=/usr/bin:/bin", "GITHUB_TOKEN=existing"}
	out := Augment(base)
	values := parseEnv(out)

	if values["PATH"] != "/usr/bin:/bin:/opt/homebrew/bin" {
		t.Fatalf("PATH = %q", values["PATH"])
	}
	if values["GH_TOKEN"] != "from-shell" {
		t.Fatalf("GH_TOKEN = %q", values["GH_TOKEN"])
	}
	if values["GITHUB_TOKEN"] != "existing" {
		t.Fatalf("GITHUB_TOKEN = %q", values["GITHUB_TOKEN"])
	}
}

func TestAugmentKeepsOrderAndDedupes(t *testing.T) {
	stubShell(t, map[string]string{"PATH": "/usr/bin:/custom/bin"})

	base := []string{"FIRST=1", "PATH=/usr/bin:/bin", "LAST=2"}
	out := Augment(base)
	if out[0] != "FIRST=1" || out[1] != "PATH=/usr/bin:/bin:/custom/bin" || out[2] != "LAST=2" {
		t.Fatalf("order = %#v", out)
	}
}

func TestApplyReportsChangedKeysOnly(t *testing.T) {
	stubShell(t, map[string]string{
		"PATH":     "/custom/bin",
		"GH_TOKEN": "secret-value",
	})
	t.Setenv("PATH", "/usr/bin")
	os.Unsetenv("GH_TOKEN")
	t.Cleanup(func() { os.Unsetenv("GH_TOKEN") })

	changed := Apply()
	joined := strings.Join(changed, ",")
	if !strings.Contains(joined, "PATH") || !strings.Contains(joined, "GH_TOKEN") {
		t.Fatalf("changed = %v", changed)
	}
	for _, key := range changed {
		if key == "secret-value" || strings.Contains(key, "secret-value") {
			t.Fatalf("Apply leaked a value: %v", changed)
		}
	}
	if got := os.Getenv("GH_TOKEN"); got != "secret-value" {
		t.Fatalf("GH_TOKEN not applied: %q", got)
	}
}

func TestApplyDisabledByEnv(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("PEGG_SHELL_ENV", "0")

	called := false
	orig := shellEnvFunc
	shellEnvFunc = func(context.Context) (map[string]string, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() { shellEnvFunc = orig })

	if got := Augment([]string{"PATH=/bin"}); len(got) != 1 || got[0] != "PATH=/bin" {
		t.Fatalf("Augment = %#v", got)
	}
	if called {
		t.Fatal("shell should not run when PEGG_SHELL_ENV=0")
	}
}

func TestParseShellOutputStripsBanner(t *testing.T) {
	raw := []byte("welcome to zsh\nPATH=/usr/bin:/bin\x00GH_TOKEN=abc\x00junk\x00")
	values := parseShellOutput(raw)
	if values["PATH"] != "/usr/bin:/bin" {
		t.Fatalf("PATH = %q", values["PATH"])
	}
	if values["GH_TOKEN"] != "abc" {
		t.Fatalf("GH_TOKEN = %q", values["GH_TOKEN"])
	}
	if _, ok := values["junk"]; ok {
		t.Fatal("invalid entry kept")
	}
}

func TestParseShellOutputEmpty(t *testing.T) {
	if values := parseShellOutput(nil); len(values) != 0 {
		t.Fatalf("values = %#v", values)
	}
}

func TestMergePathUsesPlatformSeparator(t *testing.T) {
	sep := string(os.PathListSeparator)
	got := mergePath("a"+sep+"b", "b"+sep+"c")
	want := "a" + sep + "b" + sep + "c"
	if got != want {
		t.Fatalf("mergePath = %q, want %q", got, want)
	}
}
