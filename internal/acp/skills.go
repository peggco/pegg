package acp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/skill"
)

type skillsReloadResult struct {
	Commands []AvailableCommand `json:"commands"`
}

type skillsReloadParams struct {
	Workspace string `json:"workspace,omitempty"`
}

func (s *Server) handleSkillsReload(_ context.Context, params json.RawMessage) (any, error) {
	var req skillsReloadParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	var err error
	if req.Workspace != "" {
		err = skill.ReloadForProject(req.Workspace)
	} else {
		err = skill.Reload()
	}
	if err != nil {
		return nil, err
	}
	commands := availableCommands()
	s.mu.RLock()
	ids := make([]string, 0, len(s.active))
	for id := range s.active {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	for _, id := range ids {
		s.notifyCommands(id)
	}
	return skillsReloadResult{Commands: commands}, nil
}

type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
	WhenToUse   string `json:"whenToUse,omitempty"`
}

type skillsListResult struct {
	Skills []skillInfo `json:"skills"`
}

func (s *Server) handleSkillsList(_ context.Context, _ json.RawMessage) (any, error) {
	skills := skill.List()
	out := make([]skillInfo, 0, len(skills))
	for _, sk := range skills {
		out = append(out, skillInfo{
			Name:        sk.Name,
			Description: sk.Description,
			Source:      sk.Source,
			Path:        sk.Path,
			WhenToUse:   sk.WhenToUse,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return skillsListResult{Skills: out}, nil
}

type skillsSearchParams struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Filter string `json:"filter,omitempty"`
	Cursor int    `json:"cursor,omitempty"`
}

type skillsSearchResult struct {
	Items      []catalog.RemoteSkill `json:"items"`
	NextCursor *int                  `json:"nextCursor,omitempty"`
}

func (s *Server) handleSkillsSearch(ctx context.Context, params json.RawMessage) (any, error) {
	var req skillsSearchParams
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
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := catalog.NewClient().Search(ctx, catalog.KindSkill, opts)
	if err != nil {
		return nil, err
	}
	return skillsSearchResult{Items: res.Items, NextCursor: res.NextCursor}, nil
}

type skillsInstallParams struct {
	Source  string `json:"source"`
	SkillID string `json:"skillId"`
	Scope   string `json:"scope,omitempty"`
}

type skillsInstallResult struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Commands []AvailableCommand `json:"commands"`
}

func (s *Server) handleSkillsInstall(ctx context.Context, params json.RawMessage) (any, error) {
	var req skillsInstallParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	dir, err := s.skillsDir(req.Scope)
	if err != nil {
		return nil, err
	}
	spec := catalog.SkillSpec{Source: strings.TrimSpace(req.Source), SkillID: strings.TrimSpace(req.SkillID)}
	if spec.Source == "" || spec.SkillID == "" {
		return nil, errors.New("source and skillId are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	installed, err := catalog.InstallSkill(ctx, catalog.NewClient(), spec, dir)
	if err != nil {
		return nil, err
	}
	s.reloadSkills()
	return skillsInstallResult{Name: installed.Name, Path: installed.Path, Commands: availableCommands()}, nil
}

func (s *Server) skillsDir(scope string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "global":
		return config.GetConfigPath("skills")
	case "project":
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(wd, ".pegg", "skills"), nil
	default:
		return "", errors.New("scope must be global or project")
	}
}

func (s *Server) reloadSkills() {
	wd, err := os.Getwd()
	if err != nil {
		_ = skill.Reload()
		return
	}
	_ = skill.ReloadForProject(wd)
	s.mu.RLock()
	ids := make([]string, 0, len(s.active))
	for id := range s.active {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	for _, id := range ids {
		s.notifyCommands(id)
	}
}
