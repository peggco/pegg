package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin"
)

func pluginReleaseServer(t *testing.T, tag string, data []byte) *httptest.Server {
	t.Helper()
	asset := "testplug_" + runtime.GOOS + "_" + runtime.GOARCH
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/latest", "/repos/owner/repo/releases/tags/" + tag:
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q,"size":%d}]}`,
				tag, asset, server.URL+"/asset", len(data))
		case "/asset":
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestPluginsInstallListExcludeRemove(t *testing.T) {
	server := pluginReleaseServer(t, "v1.0.0", []byte("plugin-v1"))
	defer server.Close()
	t.Setenv("PEGG_GITHUB_API", server.URL)

	c, _ := newTestCLI(t)
	var out bytes.Buffer
	c.root.SetOut(&out)
	c.root.SetErr(&out)

	if err := c.Execute([]string{"plugins", "install", "owner/repo", "--name", "testplug", "--skip-checksum"}); err != nil {
		t.Fatalf("install: %v\n%s", err, out.String())
	}
	dir, err := plugin.GetPluginDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "testplug")
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("installed file: %v %v", info, err)
	}

	out.Reset()
	if err := c.Execute([]string{"plugins", "list"}); err != nil {
		t.Fatalf("list: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "testplug") || !strings.Contains(out.String(), "owner/repo") {
		t.Fatalf("list output: %s", out.String())
	}

	out.Reset()
	if err := c.Execute([]string{"plugin", "list"}); err == nil {
		t.Fatal("singular plugin command should be removed")
	}

	if err := c.Execute([]string{"plugins", "exclude", "testplug"}); err != nil {
		t.Fatalf("exclude: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(cfg.Plugins.Exclude, "testplug") || !containsString(cfg.Plugins.Exclude, "owner/repo") {
		t.Fatalf("exclude config: %#v", cfg.Plugins.Exclude)
	}
	out.Reset()
	if err := c.Execute([]string{"plugins", "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "excluded") {
		t.Fatalf("excluded status: %s", out.String())
	}
	if err := c.Execute([]string{"plugins", "include", "testplug"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if containsString(cfg.Plugins.Exclude, "testplug") {
		t.Fatalf("include failed: %#v", cfg.Plugins.Exclude)
	}

	if err := c.Execute([]string{"plugins", "remove", "testplug", "--yes"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still present: %v", err)
	}
}

func TestPluginsUpdateCheck(t *testing.T) {
	server := pluginReleaseServer(t, "v1.0.0", []byte("v1"))
	defer server.Close()
	t.Setenv("PEGG_GITHUB_API", server.URL)
	c, _ := newTestCLI(t)
	var out bytes.Buffer
	c.root.SetOut(&out)
	c.root.SetErr(&out)
	if err := c.Execute([]string{"plugins", "install", "owner/repo", "--name", "testplug", "--skip-checksum"}); err != nil {
		t.Fatal(err)
	}
	server.Close()
	server = pluginReleaseServer(t, "v1.1.0", []byte("v2"))
	defer server.Close()
	t.Setenv("PEGG_GITHUB_API", server.URL)
	out.Reset()
	if err := c.Execute([]string{"plugins", "update", "testplug", "--check"}); err != nil {
		t.Fatalf("update check: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Update available") || !strings.Contains(out.String(), "1.0.0") || !strings.Contains(out.String(), "1.1.0") {
		t.Fatalf("update output: %s", out.String())
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
