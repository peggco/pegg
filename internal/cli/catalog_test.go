package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/mcp"
)

const testMCPSearchJSON = `{
  "items": [
    {"id":"github","name":"GitHub","description":"Issues and PRs.","isOfficial":true,"repository":"github/github-mcp-server","version":"1.2.0","tags":["git"],"transports":["stdio"]}
  ],
  "nextCursor": 1
}`

const testMCPEntryJSON = `{
  "id": "github",
  "name": "GitHub",
  "description": "Issues, pull requests, repositories and code search.",
  "vendor": "GitHub",
  "homepage": "https://github.com/github/github-mcp-server",
  "repository": "github/github-mcp-server",
  "license": "MIT",
  "version": "1.2.0",
  "isOfficial": true,
  "tags": ["git", "issues"],
  "transport": {
    "type": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}
  },
  "requiredEnv": ["GITHUB_TOKEN"],
  "readme": {"type": "github", "source": "github/github-mcp-server"}
}`

const testRemoteMCPEntryJSON = `{
  "id": "remote",
  "name": "Remote",
  "transport": {
    "type": "http",
    "url": "https://example.com/mcp",
    "headers": {"Authorization": "Bearer ${TOKEN}"}
  },
  "requiredEnv": ["TOKEN"]
}`

const testPluginSearchJSON = `{
  "items": [
    {"id":"peggco/pegg-browser-use","name":"browser-use","description":"Drive a real browser.","isOfficial":true,"repository":"peggco/pegg-browser-use","version":"0.3.1","tags":["browser"],"capabilities":["tools","skills"],"platforms":["linux","darwin"]}
  ],
  "nextCursor": null
}`

const testPluginEntryJSON = `{
  "id": "peggco/pegg-browser-use",
  "name": "browser-use",
  "description": "Drive a real Chromium browser through agent tools.",
  "repository": "peggco/pegg-browser-use",
  "homepage": "https://github.com/peggco/pegg-browser-use",
  "license": "MIT",
  "isOfficial": true,
  "version": "0.3.1",
  "requires": {"pegg": ">=0.3.0", "protocol": 3},
  "tags": ["browser", "automation"],
  "capabilities": ["tools", "skills"],
  "platforms": ["linux", "darwin"],
  "arch": ["amd64", "arm64"],
  "readme": {"type": "github", "source": "peggco/pegg-browser-use"}
}`

func newCatalogTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mcp":
			_, _ = fmt.Fprint(w, testMCPSearchJSON)
		case "/mcp/github":
			_, _ = fmt.Fprint(w, testMCPEntryJSON)
		case "/mcp/remote":
			_, _ = fmt.Fprint(w, testRemoteMCPEntryJSON)
		case "/plugins":
			_, _ = fmt.Fprint(w, testPluginSearchJSON)
		case "/plugins/peggco/pegg-browser-use":
			_, _ = fmt.Fprint(w, testPluginEntryJSON)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("PEGG_CATALOG_API", srv.URL)
	return srv
}

func TestMCPSearchCommandJSON(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"mcp", "search", "github", "--json"}); err != nil {
		t.Fatal(err)
	}

	var items []catalog.Entry
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(items) != 1 || items[0].ID != "github" || items[0].Name != "GitHub" {
		t.Fatalf("items = %+v", items)
	}
}

func TestMCPSearchCommandTable(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"mcp", "search", "github"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"ID", "NAME", "TRANSPORT", "OFFICIAL", "github", "GitHub", "stdio", "yes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestMCPInfoCommand(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)
	t.Setenv("GITHUB_TOKEN", "")

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"mcp", "info", "github"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"GitHub", "github/github-mcp-server", "GITHUB_TOKEN", "pegg mcp install github", "stdio"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestMCPInstallDryRunRedactsSecrets(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{
		"mcp", "install", "github",
		"--global",
		"--dry-run",
		"--env", "GITHUB_TOKEN=super-secret",
	}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "super-secret") {
		t.Fatalf("dry run leaked secret:\n%s", text)
	}
	for _, want := range []string{"mcp_servers", "${GITHUB_TOKEN}", "npx"} {
		if !strings.Contains(text, want) {
			t.Fatalf("dry run missing %q:\n%s", want, text)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.MCPServers["github"]; ok {
		t.Fatalf("dry run wrote server: %+v", cfg.MCPServers)
	}
}

func TestMCPInstallDryRunWithoutValues(t *testing.T) {
	newCatalogTestServer(t)
	t.Setenv("GITHUB_TOKEN", "")
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"mcp", "install", "github", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "${GITHUB_TOKEN}") {
		t.Fatalf("dry run should keep placeholders:\n%s", out.String())
	}
}

func TestMCPInstallDryRunRedactsHeaderOverrides(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{
		"mcp", "install", "remote",
		"--dry-run",
		"--header", "Authorization=Bearer topsecret",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "topsecret") {
		t.Fatalf("dry run leaked header secret:\n%s", out.String())
	}
}

func TestMCPInstallProject(t *testing.T) {
	newCatalogTestServer(t)
	dir := t.TempDir()
	t.Chdir(dir)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{
		"mcp", "install", "github",
		"--project",
		"--name", "gh",
		"--env", "GITHUB_TOKEN=secret-token",
		"--yes",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "secret-token") {
		t.Fatalf("install output leaked secret:\n%s", out.String())
	}

	servers, err := mcp.LoadProjectServers(dir)
	if err != nil {
		t.Fatal(err)
	}
	server, ok := servers["gh"]
	if !ok {
		t.Fatalf("servers = %+v, want gh", servers)
	}
	if server.Command != "npx" || len(server.Args) != 2 || server.Env["GITHUB_TOKEN"] != "secret-token" {
		t.Fatalf("server = %+v", server)
	}

	info, err := os.Stat(filepath.Join(dir, mcp.ProjectConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("project config mode = %o, want 600", perm)
	}
}

func TestMCPInstallMissingValuesNonTTY(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	err := c.Execute([]string{"mcp", "install", "github", "--global", "--yes"})
	if err == nil {
		t.Fatal("expected error for missing values")
	}
	text := err.Error()
	if !strings.Contains(text, "GITHUB_TOKEN") || !strings.Contains(text, "--env") {
		t.Fatalf("error = %q", text)
	}
}

func TestPluginsSearchCommandJSON(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"plugins", "search", "browser", "--json"}); err != nil {
		t.Fatal(err)
	}
	var items []catalog.Entry
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(items) != 1 || items[0].ID != "peggco/pegg-browser-use" {
		t.Fatalf("items = %+v", items)
	}
}

func TestPluginsCatalogInfoCommand(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"plugins", "info", "peggco/pegg-browser-use"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"browser-use", "0.3.1", "tools, skills", "linux, darwin", "pegg plugins install peggco/pegg-browser-use"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestPluginsInfoWarnsAboutCompatibility(t *testing.T) {
	newCatalogTestServer(t)
	c, _ := newTestCLI(t)

	var out bytes.Buffer
	c.root.SetOut(&out)
	if err := c.Execute([]string{"plugins", "info", "peggco/pegg-browser-use"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Pegg:", ">=0.3.0", "Protocol:", "warning:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
