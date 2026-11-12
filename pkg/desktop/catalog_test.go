//go:build gui

package main

import (
	"testing"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
)

func TestToCatalogItemSkillID(t *testing.T) {
	item := toCatalogItem(catalog.Entry{
		Kind:    catalog.KindSkill,
		Name:    "Example",
		Source:  "owner/repo",
		SkillID: "example",
	})

	if item.ID != "owner/repo/example" {
		t.Fatalf("id = %q", item.ID)
	}
	if item.Source != "owner/repo" {
		t.Fatalf("source = %q", item.Source)
	}
}

func TestToCatalogItemTransportSummaries(t *testing.T) {
	item := toCatalogItem(catalog.Entry{
		Kind:      catalog.KindMCP,
		ID:        "github",
		Name:      "GitHub",
		Transport: catalog.Transport{Type: "http", URL: "https://example.com/mcp"},
	})

	if item.Transport != "http" {
		t.Fatalf("transport = %q", item.Transport)
	}
	if len(item.Transports) != 1 || item.Transports[0] != "http" {
		t.Fatalf("transports = %v", item.Transports)
	}
}

func TestApplyCatalogTransport(t *testing.T) {
	cases := map[string]string{
		"stdio":           "stdio",
		"sse":             "sse",
		"http":            "streamable-http",
		"streamable-http": "streamable-http",
	}
	for input, want := range cases {
		cfg := config.MCPServerConfig{}
		applyCatalogTransport(&cfg, catalog.Entry{Transport: catalog.Transport{Type: input}})
		if cfg.Type != want {
			t.Fatalf("%s: type = %q, want %q", input, cfg.Type, want)
		}
	}
}

func TestCatalogReadmeSource(t *testing.T) {
	if got := catalogReadmeSource(catalog.Entry{Kind: catalog.KindSkill, Source: "owner/repo"}); got != "owner/repo" {
		t.Fatalf("skill source = %q", got)
	}
	if got := catalogReadmeSource(catalog.Entry{
		Kind:       catalog.KindMCP,
		Readme:     &catalog.ReadmeRef{Type: "github", Source: "owner/repo"},
		Repository: "other/repo",
	}); got != "owner/repo" {
		t.Fatalf("mcp source = %q", got)
	}
	if got := catalogReadmeSource(catalog.Entry{Kind: catalog.KindPlugin, Repository: "owner/repo"}); got != "owner/repo" {
		t.Fatalf("plugin source = %q", got)
	}
}

func TestCatalogInstallCommand(t *testing.T) {
	if got := catalogInstallCommand(catalog.Entry{Kind: catalog.KindMCP, ID: "github"}); got != "pegg mcp install github" {
		t.Fatalf("mcp command = %q", got)
	}
	if got := catalogInstallCommand(catalog.Entry{Kind: catalog.KindPlugin, ID: "owner/repo"}); got != "pegg plugins install owner/repo" {
		t.Fatalf("plugin command = %q", got)
	}
	if got := catalogInstallCommand(catalog.Entry{Kind: catalog.KindSkill, Source: "owner/repo", SkillID: "x"}); got != "pegg skills add owner/repo/x" {
		t.Fatalf("skill command = %q", got)
	}
}

func TestCatalogSearchOptions(t *testing.T) {
	opts := catalogSearchOptions("official,plugins,windows")
	if opts.Official == nil || !*opts.Official {
		t.Fatal("official filter was not applied")
	}
	if opts.Platform != "windows" {
		t.Fatalf("platform = %q", opts.Platform)
	}

	opts = catalogSearchOptions("community,stdio,darwin")
	if opts.Official == nil || *opts.Official {
		t.Fatal("community filter was not applied")
	}
	if opts.Transport != "stdio" {
		t.Fatalf("transport = %q", opts.Transport)
	}
	if opts.Platform != "darwin" {
		t.Fatalf("platform = %q", opts.Platform)
	}

	opts = catalogSearchOptions("")
	if opts.Official != nil || opts.Platform != "" || opts.Transport != "" {
		t.Fatalf("empty filter produced %+v", opts)
	}
}

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{"a", "", "b", "a"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}
