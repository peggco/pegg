package skillhub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type treeResponse struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

func (c *Client) Resolve(ctx context.Context, spec Spec) ([]byte, string, error) {
	if !isGitHubSource(spec.Source) {
		return nil, "", fmt.Errorf("%w: %s", ErrSourceUnsupported, spec.Source)
	}
	if spec.SkillID == "" {
		return nil, "", ErrBadSpec
	}
	if path := c.findSkillPath(ctx, spec); path != "" {
		if data, err := c.fetchRaw(ctx, spec.Source, path); err == nil {
			return data, path, nil
		}
	}
	for _, candidate := range fallbackPaths(spec.SkillID) {
		if data, err := c.fetchRaw(ctx, spec.Source, candidate); err == nil {
			return data, candidate, nil
		}
	}
	return nil, "", fmt.Errorf("%w: %s", ErrSkillNotFound, spec.String())
}

func (c *Client) Readme(ctx context.Context, source string) (string, error) {
	if !isGitHubSource(source) {
		return "", fmt.Errorf("%w: %s", ErrSourceUnsupported, source)
	}
	if data, err := c.fetchRaw(ctx, source, "README.md"); err == nil {
		return string(data), nil
	}
	endpoint := c.BaseURL + "/readme/" + escapeSegments(source)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("skillhub: readme request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown")
	req.Header.Set("User-Agent", "pegg")
	resp, err := c.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("skillhub: readme: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrSkillNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("skillhub: readme returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSkillBytes))
	if err != nil {
		return "", fmt.Errorf("skillhub: readme body: %w", err)
	}
	return string(data), nil
}

func (c *Client) findSkillPath(ctx context.Context, spec Spec) string {
	entries, err := c.repoTree(ctx, spec.Source)
	if err != nil {
		return ""
	}
	suffix := "/" + spec.SkillID + "/SKILL.md"
	var matches []string
	for _, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		if entry.Path == spec.SkillID+"/SKILL.md" || strings.HasSuffix(entry.Path, suffix) {
			matches = append(matches, entry.Path)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.SliceStable(matches, func(i, j int) bool {
		si, sj := skillRank(matches[i]), skillRank(matches[j])
		if si != sj {
			return si < sj
		}
		return strings.Count(matches[i], "/") < strings.Count(matches[j], "/")
	})
	return matches[0]
}

func skillRank(path string) int {
	if strings.Contains(path, "/skills/") || strings.HasPrefix(path, "skills/") {
		return 0
	}
	return 1
}

func (c *Client) repoTree(ctx context.Context, source string) ([]treeEntry, error) {
	c.mu.Lock()
	if entries, ok := c.trees[source]; ok {
		c.mu.Unlock()
		return entries, nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.GitHubAPI+"/repos/"+escapeSegments(source)+"/git/trees/HEAD?recursive=1", nil)
	if err != nil {
		return nil, fmt.Errorf("skillhub: tree request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pegg")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillhub: github tree: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skillhub: github tree returned %s", resp.Status)
	}
	var out treeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxSkillBytes)).Decode(&out); err != nil {
		return nil, fmt.Errorf("skillhub: decode github tree: %w", err)
	}
	c.mu.Lock()
	c.trees[source] = out.Tree
	c.mu.Unlock()
	return out.Tree, nil
}

func (c *Client) fetchRaw(ctx context.Context, source, path string) ([]byte, error) {
	endpoint := c.RawBase + "/" + escapeSegments(source) + "/HEAD/" + escapePath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("skillhub: raw request: %w", err)
	}
	req.Header.Set("User-Agent", "pegg")
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("skillhub: download skill: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skillhub: download skill returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSkillBytes))
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func fallbackPaths(skillID string) []string {
	return []string{
		skillID + "/SKILL.md",
		"skills/" + skillID + "/SKILL.md",
		".claude/skills/" + skillID + "/SKILL.md",
		".agents/skills/" + skillID + "/SKILL.md",
		".pegg/skills/" + skillID + "/SKILL.md",
	}
}
