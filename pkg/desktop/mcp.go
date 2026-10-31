//go:build gui

package main

import (
	"errors"
	"strings"

	"github.com/peggco/pegg/internal/core/config"
)

type MCPServerInput struct {
	Name    string            `json:"name"`
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
	if command == "" && url == "" {
		return errors.New("mcp: command or url is required")
	}
	args := make([]string, 0, len(in.Args))
	for _, arg := range in.Args {
		if v := strings.TrimSpace(arg); v != "" {
			args = append(args, v)
		}
	}
	server := config.MCPServerConfig{
		Command: command,
		Args:    args,
		Env:     in.Env,
		URL:     url,
		Headers: in.Headers,
	}
	return config.UpsertMCPServer(name, server)
}

func (a *App) RemoveMCPServer(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("mcp: name is required")
	}
	return config.RemoveMCPServer(name)
}
