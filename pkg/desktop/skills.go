//go:build gui

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/skillhub"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
)

type HubSkill struct {
	Name     string `json:"name"`
	SkillID  string `json:"skillId"`
	Source   string `json:"source"`
	URL      string `json:"url"`
	License  string `json:"license,omitempty"`
	Official bool   `json:"official"`
}

type SkillMutationResult struct {
	Path     string                   `json:"path,omitempty"`
	Commands []acpclient.SlashCommand `json:"commands,omitempty"`
}

type HubSearchResult struct {
	Items      []HubSkill `json:"items"`
	NextCursor *int       `json:"nextCursor,omitempty"`
}

func (a *App) SearchHubSkills(query string, limit int) ([]HubSkill, error) {
	items, err := skillhub.NewClient().Search(context.Background(), query, limit)
	if err != nil {
		return []HubSkill{}, err
	}
	return toHubSkills(items), nil
}

func (a *App) SearchHubSkillsFiltered(query string, limit int, filter string, cursor int) (HubSearchResult, error) {
	opts := skillhub.SearchOptions{Query: query, Limit: limit, Cursor: cursor}
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "official":
		value := true
		opts.Official = &value
	case "community":
		value := false
		opts.Official = &value
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := skillhub.NewClient().SearchPage(ctx, opts)
	if err != nil {
		return HubSearchResult{}, err
	}
	return HubSearchResult{Items: toHubSkills(res.Items), NextCursor: res.NextCursor}, nil
}

func (a *App) SkillReadme(source string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return skillhub.NewClient().Readme(ctx, strings.TrimSpace(source))
}

func (a *App) InstallHubSkill(source, skillID, scope string) (SkillMutationResult, error) {
	dir, err := a.skillDir(scope)
	if err != nil {
		return SkillMutationResult{}, err
	}
	spec := skillhub.Spec{Source: strings.TrimSpace(source), SkillID: strings.TrimSpace(skillID)}
	if spec.Source == "" || spec.SkillID == "" {
		return SkillMutationResult{}, errors.New("source and skill id are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	installed, err := skillhub.Install(ctx, skillhub.NewClient(), spec, dir)
	if err != nil {
		return SkillMutationResult{}, err
	}
	return SkillMutationResult{Path: installed.Path, Commands: a.reloadSkills()}, nil
}

func (a *App) ReloadSkills() []acpclient.SlashCommand {
	return a.reloadSkills()
}

func (a *App) skillDir(scope string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "agents":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".agents", "skills"), nil
	case "global":
		return config.GetConfigPath("skills")
	case "project":
		ws := strings.TrimSpace(a.currentWorkspace())
		if ws == "" {
			return "", errors.New("no project is open")
		}
		return filepath.Join(ws, ".pegg", "skills"), nil
	default:
		return "", fmt.Errorf("invalid skills scope: %q", scope)
	}
}

func (a *App) reloadSkills() []acpclient.SlashCommand {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil || !client.Running() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	commands, err := client.ReloadSkills(ctx, a.currentWorkspace())
	if err != nil {
		a.logf("skills: reload: %v", err)
		return nil
	}
	return commands
}

func toHubSkills(items []skillhub.RemoteSkill) []HubSkill {
	out := make([]HubSkill, 0, len(items))
	for _, item := range items {
		out = append(out, HubSkill{
			Name:     item.Name,
			SkillID:  item.SkillID,
			Source:   item.Source,
			URL:      item.URL,
			License:  item.License,
			Official: item.Official,
		})
	}
	return out
}
