package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogTargetPath(t *testing.T) {
	cases := []struct {
		kind  string
		entry catalogPublishEntry
		want  string
	}{
		{"mcp", catalogPublishEntry{ID: "github"}, "catalog/mcp/github.json"},
		{"plugin", catalogPublishEntry{Repository: "owner/repo"}, "catalog/plugins/owner/repo.json"},
		{"skill", catalogPublishEntry{Source: "owner/repo", SkillID: "x"}, "catalog/skills/owner/repo/x.json"},
	}
	for _, tc := range cases {
		got, err := catalogTargetPath(tc.kind, tc.entry)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestValidateCatalogPublish(t *testing.T) {
	valid := catalogPublishEntry{
		SchemaVersion: 1,
		Kind:          "mcp",
		ID:            "github",
		Name:          "GitHub",
		Description:   "Issues, pull requests and repository search.",
		Transport: catalogPublishTransport{
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "server"},
			Env:     map[string]string{"TOKEN": "${TOKEN}"},
		},
		RequiredEnv: []string{"TOKEN"},
	}
	if err := validateCatalogPublish("mcp", valid); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}

	missing := valid
	missing.RequiredEnv = nil
	if err := validateCatalogPublish("mcp", missing); err == nil {
		t.Fatal("expected missing placeholder error")
	}

	short := valid
	short.Description = "short"
	if err := validateCatalogPublish("mcp", short); err == nil {
		t.Fatal("expected description error")
	}

	plugin := catalogPublishEntry{
		SchemaVersion: 1,
		Kind:          "plugin",
		ID:            "owner/other",
		Name:          "Plugin",
		Description:   "A plugin with enough description text.",
		Repository:    "owner/repo",
	}
	if err := validateCatalogPublish("plugin", plugin); err == nil {
		t.Fatal("expected plugin id mismatch")
	}
}

func TestGitHubNewFileURL(t *testing.T) {
	got := githubNewFileURL("peggco/hub-registry", "main", "catalog/mcp/github.json", []byte("{\"a\":1}\n"))
	for _, want := range []string{
		"https://github.com/peggco/hub-registry/new/main/catalog/mcp/github.json",
		"filename=github.json",
		"value=%7B%22a%22%3A1%7D",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("url %q does not contain %q", got, want)
		}
	}
}

func TestCatalogPublishCommandPrintsURL(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "entry.json")
	body := `{
  "schemaVersion": 1,
  "kind": "mcp",
  "id": "github",
  "name": "GitHub",
  "description": "Issues, pull requests and repository search.",
  "transport": {"type": "stdio", "command": "npx"},
  "readme": {"type": "none"}
}`
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := (&CLI{}).newCatalogPublishCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--file", file})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Target: catalog/mcp/github.json") {
		t.Fatalf("output = %q", text)
	}
	if !strings.Contains(text, "readme") {
		t.Fatalf("output dropped unknown fields: %q", text)
	}
	if strings.Contains(text, "skillId") {
		t.Fatalf("output injected empty fields: %q", text)
	}
	if !strings.Contains(text, "github.com/peggco/hub-registry/new/main/catalog/mcp/github.json") {
		t.Fatalf("output = %q", text)
	}
}
