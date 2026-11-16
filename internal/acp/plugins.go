package acp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin/install"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type pluginInfo struct {
	Name        string   `json:"name"`
	Path        string   `json:"path,omitempty"`
	Version     string   `json:"version,omitempty"`
	Source      string   `json:"source,omitempty"`
	Description string   `json:"description,omitempty"`
	Enabled     bool     `json:"enabled"`
	Loaded      bool     `json:"loaded"`
	Manual      bool     `json:"manual,omitempty"`
	InstalledAt string   `json:"installedAt,omitempty"`
	LastError   string   `json:"lastError,omitempty"`
	EnvKeys     []string `json:"envKeys,omitempty"`
}

type pluginListResult struct {
	Plugins        []pluginInfo `json:"plugins"`
	PluginsEnabled bool         `json:"pluginsEnabled"`
}

func (s *Server) pluginDir() (string, error) {
	dir, err := config.GetConfigPath("plugins")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("plugin directory is not configured")
	}
	return dir, nil
}

func (s *Server) pluginLoaded(name string) bool {
	if s.plugins == nil {
		return false
	}
	if _, ok := s.plugins.GetPlugin(name); ok {
		return true
	}
	for _, instance := range s.plugins.ListPlugins() {
		if strings.EqualFold(instance.Name, name) {
			return true
		}
	}
	return false
}

func (s *Server) handlePluginsList(_ context.Context, _ json.RawMessage) (any, error) {
	dir, err := s.pluginDir()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(cfg.Plugins.Exclude))
	for _, name := range cfg.Plugins.Exclude {
		excluded[name] = true
	}
	installed, err := install.List(dir)
	if err != nil {
		return nil, err
	}
	loadErrors := map[string]string{}
	if s.plugins != nil {
		loadErrors = s.plugins.LoadErrors()
	}
	plugins := make([]pluginInfo, 0, len(installed))
	for _, item := range installed {
		info := pluginInfo{
			Name:      item.Name,
			Path:      item.Path,
			Version:   item.Version,
			Source:    item.Source,
			Enabled:   cfg.Plugins.Enabled && !excluded[item.Name],
			Loaded:    s.pluginLoaded(item.Name),
			Manual:    item.Manual,
			LastError: loadErrors[item.Name],
		}
		if !item.InstalledAt.IsZero() {
			info.InstalledAt = item.InstalledAt.UTC().Format(time.RFC3339)
		}
		if options, optErr := config.PluginOptions(item.Name); optErr == nil {
			for key := range options {
				info.EnvKeys = append(info.EnvKeys, key)
			}
			sort.Strings(info.EnvKeys)
		}
		plugins = append(plugins, info)
	}
	return pluginListResult{Plugins: plugins, PluginsEnabled: cfg.Plugins.Enabled}, nil
}

type pluginsSearchParams struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Filter string `json:"filter,omitempty"`
	Cursor int    `json:"cursor,omitempty"`
}

type pluginsSearchResult struct {
	Items      []catalog.PluginCatalogItem `json:"items"`
	NextCursor *int                        `json:"nextCursor,omitempty"`
}

func (s *Server) handlePluginsSearch(ctx context.Context, params json.RawMessage) (any, error) {
	var req pluginsSearchParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	opts := catalog.SearchOptions{Query: req.Query, Limit: req.Limit, Cursor: req.Cursor}
	switch strings.ToLower(strings.TrimSpace(req.Filter)) {
	case "official":
		value := true
		opts.Official = &value
	case "community":
		value := false
		opts.Official = &value
	}
	dir, err := s.pluginDir()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	items, next, err := catalog.PluginItems(ctx, catalog.NewClient(), opts, dir)
	if err != nil {
		return nil, err
	}
	return pluginsSearchResult{Items: items, NextCursor: next}, nil
}

type pluginMutationParams struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

type pluginMutationResult struct {
	Name             string   `json:"name,omitempty"`
	Path             string   `json:"path,omitempty"`
	RestartRequired  bool     `json:"restartRequired,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	CatalogVersion   string   `json:"catalogVersion,omitempty"`
	InstalledVersion string   `json:"installedVersion,omitempty"`
}

func (s *Server) pluginInstallRequest(params json.RawMessage) (install.Source, string, error) {
	var req pluginMutationParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		return install.Source{}, "", errors.New("source is required")
	}
	spec, err := install.ParseSource(source)
	if err != nil {
		return install.Source{}, "", err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = spec.DefaultName()
	}
	if err := install.ValidateName(name); err != nil {
		return install.Source{}, "", err
	}
	return spec, name, nil
}

func (s *Server) pluginWarnings(ctx context.Context, source string) ([]string, *catalog.Entry) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	entry, err := catalog.NewClient().Get(ctx, catalog.KindPlugin, source)
	if err != nil {
		return []string{"catalog entry not found; compatibility not checked"}, nil
	}
	return entry.CompatibilityWarnings(config.AppVersion, pkgplugin.ProtocolVersion), &entry
}

func (s *Server) handlePluginsInstall(ctx context.Context, params json.RawMessage) (any, error) {
	spec, name, err := s.pluginInstallRequest(params)
	if err != nil {
		return nil, err
	}
	warnings, entry := s.pluginWarnings(ctx, spec.Slug())
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	result, err := install.Install(ctx, install.NewClient(), install.Options{Source: spec, Name: name})
	if err != nil {
		return nil, err
	}
	res := pluginMutationResult{
		Name:             result.Name,
		Path:             result.Path,
		RestartRequired:  true,
		Warnings:         warnings,
		InstalledVersion: result.Version,
	}
	if entry != nil {
		res.CatalogVersion = entry.Version
		if entry.Version != "" && result.Version != "" && !catalog.SameVersion(entry.Version, result.Version) {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"catalog lists version %s, installed %s; catalog may be stale", entry.Version, result.Version))
		}
	}
	return res, nil
}

func (s *Server) handlePluginsUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	spec, name, err := s.pluginInstallRequest(params)
	if err != nil {
		return nil, err
	}
	warnings, entry := s.pluginWarnings(ctx, spec.Slug())
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	result, err := install.Update(ctx, install.NewClient(), install.Options{Source: spec, Name: name})
	if err != nil {
		return nil, err
	}
	res := pluginMutationResult{
		Name:             result.Name,
		Path:             result.Path,
		RestartRequired:  result.Updated,
		Warnings:         warnings,
		InstalledVersion: result.Version,
	}
	if entry != nil {
		res.CatalogVersion = entry.Version
	}
	return res, nil
}

type pluginRemoveParams struct {
	Name string `json:"name"`
}

func (s *Server) handlePluginsRemove(_ context.Context, params json.RawMessage) (any, error) {
	var req pluginRemoveParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	dir, err := s.pluginDir()
	if err != nil {
		return nil, err
	}
	installed, err := install.Resolve(dir, name)
	if err != nil {
		return nil, err
	}
	if err := install.Remove(dir, name); err != nil {
		return nil, err
	}
	return pluginMutationResult{Name: installed.Name, RestartRequired: true}, nil
}

type pluginToggleParams struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func (s *Server) handlePluginsToggle(_ context.Context, params json.RawMessage) (any, error) {
	var req pluginToggleParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	dir, err := s.pluginDir()
	if err != nil {
		return nil, err
	}
	installed, err := install.Resolve(dir, name)
	if err != nil {
		return nil, err
	}
	if req.Enabled {
		_, err = config.IncludePlugin(installed.Name)
	} else {
		_, err = config.ExcludePlugin(installed.Name)
	}
	if err != nil {
		return nil, err
	}
	return pluginMutationResult{Name: installed.Name, RestartRequired: true}, nil
}

type pluginEnvParams struct {
	Name string            `json:"name"`
	Env  map[string]string `json:"env"`
}

func (s *Server) handlePluginsEnv(_ context.Context, params json.RawMessage) (any, error) {
	var req pluginEnvParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	dir, err := s.pluginDir()
	if err != nil {
		return nil, err
	}
	installed, err := install.Resolve(dir, name)
	if err != nil {
		return nil, err
	}
	for key := range req.Env {
		if !validEnvKey(key) {
			return nil, fmt.Errorf("invalid environment variable name %q", key)
		}
	}
	if err := config.SetPluginOptions(installed.Name, req.Env); err != nil {
		return nil, err
	}
	return pluginMutationResult{Name: installed.Name, RestartRequired: true}, nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
