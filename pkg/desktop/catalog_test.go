//go:build gui

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin/install"
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

func TestCatalogUpdateAvailable(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		catalog   string
		want      bool
	}{
		{"newer catalog", "1.2.0", "1.3.0", true},
		{"v prefix", "v1.2.0", "1.3.0", true},
		{"equal", "1.3.0", "1.3.0", false},
		{"installed newer", "1.4.0", "1.3.0", false},
		{"missing installed", "", "1.3.0", false},
		{"missing catalog", "1.2.0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := catalogUpdateAvailable(tc.installed, tc.catalog); got != tc.want {
				t.Fatalf("catalogUpdateAvailable(%q, %q) = %v, want %v", tc.installed, tc.catalog, got, tc.want)
			}
		})
	}
}

func TestEnrichCatalogItemsFromDir(t *testing.T) {
	dir := t.TempDir()
	writeInstalledPlugin(t, dir, "browser-use", "peggco/pegg-browser-use", "1.2.0")

	items := enrichCatalogItemsFromDir([]CatalogItem{
		{Kind: catalog.KindPlugin, ID: "peggco/pegg-browser-use", Name: "browser-use", Source: "peggco/pegg-browser-use", Version: "1.3.0"},
		{Kind: catalog.KindPlugin, ID: "peggco/other", Name: "other", Source: "peggco/other", Version: "2.0.0"},
		{Kind: catalog.KindSkill, ID: "owner/repo/skill", Name: "skill", Version: "9.9.9"},
	}, dir)

	if items[0].InstalledVersion != "1.2.0" || !items[0].UpdateAvailable {
		t.Fatalf("installed plugin = %+v", items[0])
	}
	if items[1].InstalledVersion != "" || items[1].UpdateAvailable {
		t.Fatalf("uninstalled plugin = %+v", items[1])
	}
	if items[2].InstalledVersion != "" || items[2].UpdateAvailable {
		t.Fatalf("skill = %+v", items[2])
	}
}

func TestEnrichCatalogItemsNoDowngrade(t *testing.T) {
	dir := t.TempDir()
	writeInstalledPlugin(t, dir, "browser-use", "peggco/pegg-browser-use", "1.4.0")

	items := enrichCatalogItemsFromDir([]CatalogItem{
		{Kind: catalog.KindPlugin, ID: "peggco/pegg-browser-use", Name: "browser-use", Version: "1.3.0"},
	}, dir)

	if items[0].InstalledVersion != "1.4.0" || items[0].UpdateAvailable {
		t.Fatalf("downgrade offered: %+v", items[0])
	}
}

func writeInstalledPlugin(t *testing.T, dir, name, source, version string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := install.WriteMetadata(dir, &install.Metadata{Name: name, Source: source, Version: version}); err != nil {
		t.Fatal(err)
	}
}
