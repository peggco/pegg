package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

const ProjectConfigFileName = ".mcp.json"

type projectConfig struct {
	MCPServers map[string]config.MCPServerConfig `json:"mcpServers"`
}

func LoadProjectServers(dir string) (map[string]config.MCPServerConfig, error) {
	path := filepath.Join(dir, ProjectConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]config.MCPServerConfig{}, nil
		}
		return nil, fmt.Errorf("mcp: read %s: %w", path, err)
	}

	var cfg projectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mcp: parse %s: %w", path, err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]config.MCPServerConfig{}
	}
	return cfg.MCPServers, nil
}

func MergeServers(global, project map[string]config.MCPServerConfig) map[string]config.MCPServerConfig {
	out := make(map[string]config.MCPServerConfig, len(global)+len(project))
	for name, cfg := range global {
		out[name] = cfg
	}
	for name, cfg := range project {
		out[name] = cfg
	}
	return out
}

func ConfiguredServers(global map[string]config.MCPServerConfig) (map[string]config.MCPServerConfig, error) {
	project, err := LoadProjectServers(".")
	if err != nil {
		return nil, err
	}
	return MergeServers(global, project), nil
}

func ProjectConfigPath(dir string) string {
	return filepath.Join(dir, ProjectConfigFileName)
}

func SaveProjectServers(dir string, servers map[string]config.MCPServerConfig) error {
	path := ProjectConfigPath(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mcp: create dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(projectConfig{MCPServers: servers}, "", "  ")
	if err != nil {
		return fmt.Errorf("mcp: marshal project servers: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".mcp-*.json")
	if err != nil {
		return fmt.Errorf("mcp: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("mcp: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("mcp: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("mcp: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("mcp: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("mcp: replace %s: %w", path, err)
	}
	return nil
}

func UpsertProjectServer(dir, name string, server config.MCPServerConfig) error {
	servers, err := LoadProjectServers(dir)
	if err != nil {
		return err
	}
	servers[name] = server
	return SaveProjectServers(dir, servers)
}

func RemoveProjectServer(dir, name string) error {
	servers, err := LoadProjectServers(dir)
	if err != nil {
		return err
	}
	if _, ok := servers[name]; !ok {
		return fmt.Errorf("mcp server %q not found in project", name)
	}
	delete(servers, name)
	return SaveProjectServers(dir, servers)
}
