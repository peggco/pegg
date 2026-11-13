package loadskill

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/builtin/tools/subagent"
	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/plugin/bundle"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/vfs"
)

const (
	contextFork      = "fork"
	hubInstallTime   = 60 * time.Second
	hubSearchResults = 20
)

func generateLoadSkillToolPrompt() (string, error) {
	sys, err := loadSkillPromptBuilder().
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}

type loadSkillParams struct {
	Name      string            `json:"name"`
	Source    string            `json:"source,omitempty"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type agentSessions struct {
	mu       sync.Mutex
	sessions map[string]string
}

var sessions = &agentSessions{sessions: make(map[string]string)}

func (s *agentSessions) subscribe(bus event.Bus) {
	if bus == nil {
		return
	}
	_ = bus.Subscribe(session.TopicSessionAttached, func(e session.SessionAttached) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.sessions[e.AgentID] = e.SessionID
	})
}

func (s *agentSessions) get(agentID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[agentID]
}

func LoadSkillTool(fs *vfs.VFS, sess *session.Manager) {
	if sess != nil {
		sessions.subscribe(sess.Bus())
	}
	tools.Register(loadSkillTool(fs, sess, catalog.NewClient()))
}

func loadSkillTool(fs *vfs.VFS, sess *session.Manager, hub *catalog.Client) tool.Tool {
	prompt, err := generateLoadSkillToolPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to generate loadskill tool prompt: %v", err))
	}

	return tool.NewSpec(
		"loadskill",
		prompt,
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the skill to load (e.g. 'batch', 'review', 'docx'), or a full hub reference like 'owner/repo/skill'.",
				},
				"source": map[string]any{
					"type":        "string",
					"description": "Optional hub source (owner/repo) when multiple skills share the same name. Ignored when the skill is already loaded.",
				},
				"arguments": map[string]any{
					"type":        "object",
					"description": "Optional arguments to substitute into the skill content. Keys correspond to $arg_name placeholders in the skill instructions.",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"name"},
		},
		func(ctx context.Context, args string) (string, error) {
			var params loadSkillParams
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("loadskill: invalid arguments: %w", err)
			}
			params.Name = strings.TrimSpace(params.Name)
			if params.Name == "" {
				return "", fmt.Errorf("loadskill: name is required")
			}
			params.Source = strings.TrimSpace(params.Source)

			s, ok := skill.Get(params.Name)
			if !ok {
				s, err = loadSkillFromHub(ctx, hub, params.Name, params.Source)
				if err != nil {
					return "", fmt.Errorf("loadskill: skill %q is not loaded: %w", params.Name, err)
				}
			}

			return renderSkill(ctx, sess, fs, s, params.Arguments)
		},
	)
}

func renderSkill(ctx context.Context, sess *session.Manager, fs *vfs.VFS, s *skill.Skill, args map[string]string) (string, error) {
	activation := bundle.Activate(ctx, bundle.Deps{FS: fs}, s.ToolBundles)
	content := skill.ExpandWithArgs(s, args)
	if s.Context == contextFork {
		return executeFork(ctx, sess, s, content)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<skill:%s>\n", s.Name)
	for _, line := range activation {
		fmt.Fprintf(&b, "%s\n", line)
	}
	if s.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", s.Description)
	}
	fmt.Fprintf(&b, "Path: %s\n", s.Path)
	if s.HasScripts {
		fmt.Fprintf(&b, "Scripts: %s\n", s.ScriptsPath)
	}
	if len(s.AllowedTools) > 0 {
		fmt.Fprintf(&b, "Allowed tools: %s\n", strings.Join(s.AllowedTools, ", "))
	}
	if s.WhenToUse != "" {
		fmt.Fprintf(&b, "When to use: %s\n", s.WhenToUse)
	}
	if s.ArgumentHint != "" {
		fmt.Fprintf(&b, "Arguments: %s\n", s.ArgumentHint)
	}
	if len(s.Arguments) > 0 {
		fmt.Fprintf(&b, "Parameters: %s\n", strings.Join(s.Arguments, ", "))
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(content))
	fmt.Fprintf(&b, "\n</skill:%s>\n", s.Name)

	return b.String(), nil
}

func loadSkillFromHub(ctx context.Context, hub *catalog.Client, name, source string) (*skill.Skill, error) {
	if hub == nil {
		return nil, fmt.Errorf("hub client is unavailable")
	}

	spec, err := resolveHubSpec(ctx, hub, name, source)
	if err != nil {
		return nil, err
	}

	dir, err := config.GetConfigPath("skills")
	if err != nil {
		return nil, fmt.Errorf("resolve global skills directory: %w", err)
	}

	installCtx, cancel := context.WithTimeout(ctx, hubInstallTime)
	defer cancel()

	installed, err := catalog.InstallSkill(installCtx, hub, spec, dir)
	if err != nil {
		return nil, fmt.Errorf("install %s from hub: %w", spec.String(), err)
	}
	if err := skill.LoadSkillDir(installed.Path, dir); err != nil {
		return nil, fmt.Errorf("load installed skill %s: %w", spec.String(), err)
	}
	if s, ok := skill.Get(installed.Name); ok {
		return s, nil
	}
	if s, ok := skill.Get(spec.SkillID); ok {
		return s, nil
	}
	return nil, fmt.Errorf("installed skill %s could not be registered", spec.String())
}

func resolveHubSpec(ctx context.Context, hub *catalog.Client, name, source string) (catalog.SkillSpec, error) {
	if source != "" {
		if spec, err := catalog.ParseSkillSpec(name); err == nil && spec.Source == source {
			return spec, nil
		}
		return catalog.SkillSpec{Source: source, SkillID: name}, nil
	}
	if strings.Contains(name, "/") {
		return catalog.ParseSkillSpec(name)
	}
	return findHubSkill(ctx, hub, name)
}

func findHubSkill(ctx context.Context, hub *catalog.Client, name string) (catalog.SkillSpec, error) {
	res, err := hub.Search(ctx, catalog.KindSkill, catalog.SearchOptions{Query: name, Limit: hubSearchResults})
	if err != nil {
		return catalog.SkillSpec{}, fmt.Errorf("search hub: %w", err)
	}
	items := res.Items

	var matches []catalog.RemoteSkill
	for _, item := range items {
		if strings.EqualFold(item.SkillID, name) || strings.EqualFold(item.Name, name) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		if len(items) > 0 {
			return catalog.SkillSpec{}, fmt.Errorf("no exact hub match; candidates: %s", hubRefs(items, 5))
		}
		return catalog.SkillSpec{}, fmt.Errorf("no hub skill matched %q", name)
	}

	if len(matches) > 1 {
		var official []catalog.RemoteSkill
		for _, item := range matches {
			if item.Official {
				official = append(official, item)
			}
		}
		if len(official) == 1 {
			matches = official
		}
	}
	if len(matches) > 1 {
		return catalog.SkillSpec{}, fmt.Errorf("multiple hub skills match %q; retry with source or ref: %s", name, hubRefs(matches, 5))
	}

	return catalog.SkillSpec{Source: matches[0].Source, SkillID: matches[0].SkillID}, nil
}

func hubRefs(items []catalog.RemoteSkill, limit int) string {
	refs := make([]string, 0, len(items))
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		refs = append(refs, item.Source+"/"+item.SkillID)
	}
	return strings.Join(refs, ", ")
}

func executeFork(ctx context.Context, sess *session.Manager, s *skill.Skill, content string) (string, error) {
	parent := agent.FromContext(ctx)
	if parent == nil {
		return "", fmt.Errorf("loadskill: no parent agent in context")
	}

	var sessionID string
	if sess != nil {
		if id := sessions.get(parent.ID); id != "" {
			msgs, err := sess.Messages(id)
			if err == nil && len(msgs) > 0 {
				forked, err := sess.Fork(id, msgs[len(msgs)-1].ID)
				if err != nil {
					return "", fmt.Errorf("loadskill: fork session: %w", err)
				}
				sessionID = forked.ID
			}
		}
	}

	name := fmt.Sprintf("%s-%s", s.Name, uuid.NewString()[:8])
	return subagent.SpawnForked(parent, name, sessionID, content)
}
