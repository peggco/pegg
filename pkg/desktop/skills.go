//go:build gui

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/skill"
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
	Name     string                   `json:"name,omitempty"`
	Path     string                   `json:"path,omitempty"`
	Commands []acpclient.SlashCommand `json:"commands,omitempty"`
}

type HubSearchResult struct {
	Items      []HubSkill `json:"items"`
	NextCursor *int       `json:"nextCursor,omitempty"`
}

func (a *App) SearchHubSkills(query string, limit int) ([]HubSkill, error) {
	res, err := catalog.NewClient().Search(context.Background(), catalog.KindSkill, catalog.SearchOptions{Query: query, Limit: limit})
	if err != nil {
		return []HubSkill{}, err
	}
	return toHubSkills(res.Items), nil
}

func (a *App) SearchHubSkillsFiltered(query string, limit int, filter string, cursor int) (HubSearchResult, error) {
	opts := catalog.SearchOptions{Query: query, Limit: limit, Cursor: cursor}
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
	res, err := catalog.NewClient().Search(ctx, catalog.KindSkill, opts)
	if err != nil {
		return HubSearchResult{}, err
	}
	return HubSearchResult{Items: toHubSkills(res.Items), NextCursor: res.NextCursor}, nil
}

func (a *App) SkillReadme(source string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return catalog.NewClient().SkillReadme(ctx, strings.TrimSpace(source))
}

func (a *App) InstallHubSkill(source, skillID, scope string) (SkillMutationResult, error) {
	dir, err := a.skillDir(scope)
	if err != nil {
		return SkillMutationResult{}, err
	}
	spec := catalog.SkillSpec{Source: strings.TrimSpace(source), SkillID: strings.TrimSpace(skillID)}
	if spec.Source == "" || spec.SkillID == "" {
		return SkillMutationResult{}, errors.New("source and skill id are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	installed, err := catalog.InstallSkill(ctx, catalog.NewClient(), spec, dir)
	if err != nil {
		return SkillMutationResult{}, err
	}
	return SkillMutationResult{Name: installed.Name, Path: installed.Path, Commands: a.reloadSkills()}, nil
}

func (a *App) ReloadSkills() []acpclient.SlashCommand {
	return a.reloadSkills()
}

func (a *App) agentSkills() []acpclient.SkillInfo {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil || !client.Running() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	skills, err := client.ListSkills(ctx)
	if err != nil {
		a.logf("skills: list: %v", err)
		return nil
	}
	return skills
}

func (a *App) agentSkillCommands() []acpclient.SlashCommand {
	skills := a.agentSkills()
	out := make([]acpclient.SlashCommand, 0, len(skills))
	for _, sk := range skills {
		description := sk.Description
		if description == "" {
			description = sk.WhenToUse
		}
		out = append(out, acpclient.SlashCommand{Name: sk.Name, Description: description})
	}
	return out
}

func mergeSkillSettings(local []*skill.Skill, remote []acpclient.SkillInfo) []SkillSetting {
	entries := make(map[string]SkillSetting, len(local)+len(remote))
	for _, item := range local {
		entries[item.Name] = SkillSetting{
			Name:        item.Name,
			Description: item.Description,
			Source:      item.Source,
			Path:        item.Path,
		}
	}
	for _, item := range remote {
		if _, exists := entries[item.Name]; exists && !strings.HasPrefix(item.Source, "plugin:") {
			continue
		}
		entries[item.Name] = SkillSetting{
			Name:        item.Name,
			Description: item.Description,
			Source:      item.Source,
			Path:        item.Path,
		}
	}
	out := make([]SkillSetting, 0, len(entries))
	for _, item := range entries {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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

func toHubSkills(items []catalog.RemoteSkill) []HubSkill {
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
