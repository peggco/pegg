//go:build gui

package main

import (
	"errors"
	"strings"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/mcp"
)

type MCPServerInput struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (a *App) AddMCPServer(in MCPServerInput) error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return errors.New("mcp: name is required")
	}
	command := strings.TrimSpace(in.Command)
	url := strings.TrimSpace(in.URL)
	args := make([]string, 0, len(in.Args))
	for _, arg := range in.Args {
		if v := strings.TrimSpace(arg); v != "" {
			args = append(args, v)
		}
	}
	server := config.MCPServerConfig{
		Type:    in.Type,
		Command: command,
		Args:    args,
		Env:     in.Env,
		URL:     url,
		Headers: in.Headers,
	}
	kind, err := mcp.ResolveTransportKind(server)
	if err != nil {
		return err
	}
	server.Type = string(kind)
	return config.UpsertMCPServer(name, server)
}

func (a *App) RemoveMCPServer(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("mcp: name is required")
	}
	return config.RemoveMCPServer(name)
}
