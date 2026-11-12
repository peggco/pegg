//go:build gui

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/mcp"
	"github.com/peggco/pegg/internal/plugin/install"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type CatalogItem struct {
	Kind         string   `json:"kind"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Source       string   `json:"source"`
	URL          string   `json:"url,omitempty"`
	License      string   `json:"license,omitempty"`
	Official     bool     `json:"official"`
	Tags         []string `json:"tags,omitempty"`
	Version      string   `json:"version,omitempty"`
	Transport    string   `json:"transport,omitempty"`
	Transports   []string `json:"transports,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	Repository   string   `json:"repository,omitempty"`
	Homepage     string   `json:"homepage,omitempty"`
	RequiredEnv  []string `json:"requiredEnv,omitempty"`
	SkillID      string   `json:"skillId,omitempty"`
}

type CatalogSearchResult struct {
	Items      []CatalogItem `json:"items"`
	NextCursor *int          `json:"nextCursor,omitempty"`
}

type CatalogEntry struct {
	CatalogItem
	ConfigJSON            string   `json:"configJSON,omitempty"`
	InstallCommand        string   `json:"installCommand,omitempty"`
	ReadmeSource          string   `json:"readmeSource,omitempty"`
	PeggRequirement       string   `json:"peggRequirement,omitempty"`
	ProtocolRequirement   int      `json:"protocolRequirement,omitempty"`
	CompatibilityWarnings []string `json:"compatibilityWarnings,omitempty"`
}

type CatalogMutationResult struct {
	Name             string                   `json:"name,omitempty"`
	Path             string                   `json:"path,omitempty"`
	RestartRequired  bool                     `json:"restartRequired,omitempty"`
	Commands         []acpclient.SlashCommand `json:"commands,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
	CatalogVersion   string                   `json:"catalogVersion,omitempty"`
	InstalledVersion string                   `json:"installedVersion,omitempty"`
}

type CatalogInstalled struct {
	Skills  []string `json:"skills"`
	MCP     []string `json:"mcp"`
	Plugins []string `json:"plugins"`
}

func (a *App) SearchCatalog(kind, query, filter string, limit, cursor int) (CatalogSearchResult, error) {
	opts := catalogSearchOptions(filter)
	opts.Query = query
	opts.Limit = limit
	opts.Cursor = cursor

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := catalog.NewClient().Search(ctx, kind, opts)
	if err != nil {
		return CatalogSearchResult{}, err
	}

	items := make([]CatalogItem, 0, len(res.Items))
	for _, entry := range res.Items {
		items = append(items, toCatalogItem(entry))
	}
	return CatalogSearchResult{Items: items, NextCursor: res.NextCursor}, nil
}

func catalogSearchOptions(filter string) catalog.SearchOptions {
	var opts catalog.SearchOptions
	for _, token := range strings.Split(strings.ToLower(filter), ",") {
		token = strings.TrimSpace(token)
		switch token {
		case "":
			continue
		case "official":
			value := true
			opts.Official = &value
		case "community":
			value := false
			opts.Official = &value
		case "stdio", "sse", "http":
			opts.Transport = token
		case "linux", "darwin", "windows":
			opts.Platform = token
		}
	}
	return opts
}

func (a *App) GetCatalogEntry(kind, id string) (CatalogEntry, error) {
	entry, err := a.catalogEntry(kind, id)
	if err != nil {
		return CatalogEntry{}, err
	}

	out := CatalogEntry{CatalogItem: toCatalogItem(entry)}
	out.ReadmeSource = catalogReadmeSource(entry)
	out.InstallCommand = catalogInstallCommand(entry)

	if entry.Kind == catalog.KindPlugin {
		if entry.Requires != nil {
			out.PeggRequirement = entry.Requires.Pegg
			out.ProtocolRequirement = entry.Requires.Protocol
		}
		out.CompatibilityWarnings = entry.CompatibilityWarnings(config.AppVersion, pkgplugin.ProtocolVersion)
	}

	if entry.Kind == catalog.KindMCP {
		cfg, _, cfgErr := entry.Config(nil)
		if cfgErr == nil {
			applyCatalogTransport(&cfg, entry)
			if data, marshalErr := json.MarshalIndent(cfg, "", "  "); marshalErr == nil {
				out.ConfigJSON = string(data)
			}
		}
	}

	return out, nil
}

func (a *App) CatalogReadme(kind, id string) (string, error) {
	entry, err := a.catalogEntry(kind, id)
	if err != nil {
		return "", err
	}
	source := catalogReadmeSource(entry)
	if source == "" {
		return "", errors.New("catalog: no GitHub source for this entry")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return catalog.NewClient().SkillReadme(ctx, source)
}

func (a *App) InstallCatalogMCP(id string, env, headers map[string]string, scope string) (CatalogMutationResult, error) {
	entry, err := a.catalogEntry(catalog.KindMCP, id)
	if err != nil {
		return CatalogMutationResult{}, err
	}

	cfg, missing, err := entry.Config(env)
	if err != nil {
		return CatalogMutationResult{}, err
	}
	if len(missing) > 0 {
		return CatalogMutationResult{}, fmt.Errorf("catalog: missing required values: %s", strings.Join(missing, ", "))
	}

	applyCatalogTransport(&cfg, entry)
	for key, value := range headers {
		if key = strings.TrimSpace(key); key == "" {
			continue
		}
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		cfg.Headers[key] = value
	}

	name := strings.TrimSpace(entry.ID)
	if name == "" {
		return CatalogMutationResult{}, errors.New("catalog: mcp entry has no id")
	}

	if strings.EqualFold(strings.TrimSpace(scope), "project") {
		dir := a.currentWorkspace()
		if err := mcp.UpsertProjectServer(dir, name, cfg); err != nil {
			return CatalogMutationResult{}, err
		}
		return CatalogMutationResult{
			Name:            name,
			Path:            mcp.ProjectConfigPath(dir),
			RestartRequired: true,
		}, nil
	}

	if err := config.UpsertMCPServer(name, cfg); err != nil {
		return CatalogMutationResult{}, err
	}
	path, _ := config.GetConfigPath("pegg.json")
	return CatalogMutationResult{Name: name, Path: path, RestartRequired: true}, nil
}

func (a *App) InstallCatalogPlugin(source, name string) (CatalogMutationResult, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return CatalogMutationResult{}, errors.New("catalog: plugin source is required")
	}
	spec, err := install.ParseSource(source)
	if err != nil {
		return CatalogMutationResult{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = spec.DefaultName()
	}

	entry, entryErr := a.catalogEntry(catalog.KindPlugin, source)
	var warnings []string
	if entryErr == nil {
		warnings = entry.CompatibilityWarnings(config.AppVersion, pkgplugin.ProtocolVersion)
	} else {
		warnings = append(warnings, "catalog entry not found; compatibility not checked")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	result, err := install.Install(ctx, install.NewClient(), install.Options{
		Source: spec,
		Name:   name,
	})
	if err != nil {
		return CatalogMutationResult{}, err
	}

	res := CatalogMutationResult{
		Name:             result.Name,
		Path:             result.Path,
		RestartRequired:  true,
		Warnings:         warnings,
		InstalledVersion: result.Version,
	}
	if entryErr == nil {
		res.CatalogVersion = entry.Version
		if entry.Version != "" && result.Version != "" &&
			!catalog.SameVersion(entry.Version, result.Version) {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"catalog lists version %s, installed %s; catalog may be stale",
				entry.Version, result.Version))
		}
	}
	return res, nil
}

func (a *App) CatalogInstalled() (CatalogInstalled, error) {
	installed := CatalogInstalled{Skills: []string{}, MCP: []string{}, Plugins: []string{}}

	globalSkills, _ := config.GetConfigPath("skills")
	if globalSkills != "" {
		for _, item := range catalog.ListInstalledSkills(globalSkills, "global") {
			installed.Skills = append(installed.Skills, skillCatalogKey(item))
		}
	}
	if ws := a.currentWorkspace(); ws != "" {
		for _, item := range catalog.ListInstalledSkills(filepath.Join(ws, ".pegg", "skills"), "project") {
			installed.Skills = append(installed.Skills, skillCatalogKey(item))
		}
	}
	installed.Skills = uniqueStrings(installed.Skills)

	if cfg, err := config.Load(); err == nil {
		for name := range cfg.MCPServers {
			installed.MCP = append(installed.MCP, name)
		}
	}
	if ws := a.currentWorkspace(); ws != "" {
		if servers, err := mcp.LoadProjectServers(ws); err == nil {
			for name := range servers {
				installed.MCP = append(installed.MCP, name)
			}
		}
	}
	installed.MCP = uniqueStrings(installed.MCP)

	pluginDir, _ := config.GetConfigPath("plugins")
	if pluginDir != "" {
		if items, err := install.List(pluginDir); err == nil {
			for _, item := range items {
				installed.Plugins = append(installed.Plugins, item.Name)
			}
		}
	}
	installed.Plugins = uniqueStrings(installed.Plugins)

	return installed, nil
}

func (a *App) catalogEntry(kind, id string) (catalog.Entry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return catalog.NewClient().Get(ctx, kind, id)
}

func toCatalogItem(entry catalog.Entry) CatalogItem {
	kind := entry.Kind
	if kind == "" {
		kind = catalog.KindSkill
	}

	id := entry.ID
	if kind == catalog.KindSkill {
		if entry.Source != "" && entry.SkillID != "" {
			id = entry.Source + "/" + entry.SkillID
		}
	}
	if id == "" {
		id = entry.Name
	}

	source := entry.Source
	if source == "" {
		source = entry.Repository
	}

	url := entry.URL
	if url == "" {
		url = entry.Homepage
	}
	if url == "" && entry.Repository != "" {
		url = "https://github.com/" + entry.Repository
	}

	transports := entry.Transports
	if len(transports) == 0 && entry.Transport.Type != "" {
		transports = []string{entry.Transport.Type}
	}

	return CatalogItem{
		Kind:         kind,
		ID:           id,
		Name:         entry.Name,
		Description:  entry.Description,
		Source:       source,
		URL:          url,
		License:      entry.License,
		Official:     entry.Official,
		Tags:         entry.Tags,
		Version:      entry.Version,
		Transport:    entry.Transport.Type,
		Transports:   transports,
		Capabilities: entry.Capabilities,
		Platforms:    entry.Platforms,
		Repository:   entry.Repository,
		Homepage:     entry.Homepage,
		RequiredEnv:  entry.RequiredEnv,
		SkillID:      entry.SkillID,
	}
}

func catalogReadmeSource(entry catalog.Entry) string {
	if entry.Kind == catalog.KindSkill {
		return entry.Source
	}
	if entry.Readme != nil && entry.Readme.Type == "github" && entry.Readme.Source != "" {
		return entry.Readme.Source
	}
	return entry.Repository
}

func catalogInstallCommand(entry catalog.Entry) string {
	switch entry.Kind {
	case catalog.KindMCP:
		return "pegg mcp install " + entry.ID
	case catalog.KindPlugin:
		return "pegg plugins install " + entry.ID
	default:
		if entry.Source != "" && entry.SkillID != "" {
			return "pegg skills add " + entry.Source + "/" + entry.SkillID
		}
		return ""
	}
}

func applyCatalogTransport(cfg *config.MCPServerConfig, entry catalog.Entry) {
	switch strings.ToLower(strings.TrimSpace(entry.Transport.Type)) {
	case "stdio", "local":
		cfg.Type = "stdio"
	case "sse":
		cfg.Type = "sse"
	case "http", "streamable", "streamable-http":
		cfg.Type = "streamable-http"
	}
}

func skillCatalogKey(item catalog.InstalledSkill) string {
	if item.Source != "" && item.SkillID != "" {
		return item.Source + "/" + item.SkillID
	}
	return item.Name
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
