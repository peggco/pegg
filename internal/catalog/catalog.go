package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIBase = "https://hub-api.pegg.dev/v1"
	DefaultGitHub  = "https://api.github.com"
	DefaultRawBase = "https://raw.githubusercontent.com"

	defaultHTTPTimeout = 20 * time.Second
	maxResponseBytes   = 2 << 20
)

const (
	KindMCP    = "mcp"
	KindPlugin = "plugins"
	KindSkill  = "skills"
)

var (
	ErrNotFound = errors.New("catalog: entry not found")
	ErrBadSpec  = errors.New("catalog: invalid entry reference")
)

type Entry struct {
	Kind        string     `json:"kind,omitempty"`
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Vendor      string     `json:"vendor,omitempty"`
	Homepage    string     `json:"homepage,omitempty"`
	Repository  string     `json:"repository,omitempty"`
	License     string     `json:"license,omitempty"`
	Version     string     `json:"version,omitempty"`
	Official    bool       `json:"isOfficial"`
	Tags        []string   `json:"tags,omitempty"`
	Transport   Transport  `json:"transport,omitempty"`
	Transports  []string   `json:"transports,omitempty"`
	RequiredEnv []string   `json:"requiredEnv,omitempty"`
	Readme      *ReadmeRef `json:"readme,omitempty"`

	Capabilities []string        `json:"capabilities,omitempty"`
	Platforms    []string        `json:"platforms,omitempty"`
	Arch         []string        `json:"arch,omitempty"`
	Install      *InstallRef     `json:"install,omitempty"`
	Requires     *PluginRequires `json:"requires,omitempty"`

	SkillID string `json:"skillId,omitempty"`
	Source  string `json:"source,omitempty"`
	URL     string `json:"url,omitempty"`
	Path    string `json:"path,omitempty"`
}

type Transport struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type ReadmeRef struct {
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
}

type PluginRequires struct {
	Pegg     string `json:"pegg,omitempty"`
	Protocol int    `json:"protocol,omitempty"`
}

type InstallRef struct {
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
	Path   string `json:"path,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type Client struct {
	BaseURL   string
	GitHubAPI string
	RawBase   string
	HTTP      *http.Client
	Token     string

	mu    sync.Mutex
	trees map[string][]treeEntry
}

func NewClient() *Client {
	base := strings.TrimSpace(os.Getenv("PEGG_CATALOG_API"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("PEGG_SKILLS_API"))
	}
	if base == "" {
		base = DefaultAPIBase
	}
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	return &Client{
		BaseURL:   strings.TrimRight(base, "/"),
		GitHubAPI: strings.TrimRight(DefaultGitHub, "/"),
		RawBase:   strings.TrimRight(DefaultRawBase, "/"),
		HTTP:      &http.Client{Timeout: defaultHTTPTimeout},
		Token:     token,
		trees:     map[string][]treeEntry{},
	}
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

type SearchOptions struct {
	Query     string
	Limit     int
	Cursor    int
	Official  *bool
	Transport string
	Platform  string
}

type SearchResult struct {
	Items      []Entry
	NextCursor *int
}

func (c *Client) Search(ctx context.Context, kind string, opts SearchOptions) (SearchResult, error) {
	canonical, err := NormalizeKind(kind)
	if err != nil {
		return SearchResult{}, err
	}

	endpoint, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + "/" + canonical)
	if err != nil {
		return SearchResult{}, fmt.Errorf("catalog: build search url: %w", err)
	}
	q := endpoint.Query()
	if query := strings.TrimSpace(opts.Query); query != "" {
		q.Set("q", query)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	q.Set("limit", strconv.Itoa(limit))
	if opts.Cursor > 0 {
		q.Set("cursor", strconv.Itoa(opts.Cursor))
	}
	if opts.Official != nil {
		q.Set("official", strconv.FormatBool(*opts.Official))
	}
	if value := strings.TrimSpace(opts.Transport); value != "" {
		q.Set("transport", value)
	}
	if value := strings.TrimSpace(opts.Platform); value != "" {
		q.Set("platform", value)
	}
	q.Set("fields", searchFields(canonical))
	endpoint.RawQuery = q.Encode()

	var out struct {
		Items      []Entry `json:"items"`
		NextCursor *int    `json:"nextCursor"`
	}
	if err := c.getJSON(ctx, endpoint.String(), &out); err != nil {
		return SearchResult{}, err
	}
	if out.Items == nil {
		out.Items = []Entry{}
	}
	for i := range out.Items {
		if out.Items[i].Kind == "" {
			out.Items[i].Kind = canonical
		}
	}
	return SearchResult{Items: out.Items, NextCursor: out.NextCursor}, nil
}

func (c *Client) Get(ctx context.Context, kind, id string) (Entry, error) {
	canonical, err := NormalizeKind(kind)
	if err != nil {
		return Entry{}, err
	}
	endpoint, err := c.entryEndpoint(canonical, id)
	if err != nil {
		return Entry{}, err
	}
	var out Entry
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return Entry{}, err
	}
	if out.Kind == "" {
		out.Kind = canonical
	}
	return out, nil
}

func (c *Client) entryEndpoint(kind, id string) (string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	switch kind {
	case KindMCP:
		mcpID, err := ParseMCPID(id)
		if err != nil {
			return "", err
		}
		return base + "/mcp/" + url.PathEscape(mcpID), nil
	case KindPlugin:
		pluginID, err := ParsePluginID(id)
		if err != nil {
			return "", err
		}
		spec, _ := ParsePluginSpec(pluginID)
		return base + "/plugins/" + url.PathEscape(spec.Owner) + "/" + url.PathEscape(spec.Repo), nil
	case KindSkill:
		source, skillID, err := ParseSkillRef(id)
		if err != nil {
			return "", err
		}
		return base + "/skills/" + escapeSegments(source) + "/" + url.PathEscape(skillID), nil
	default:
		return "", fmt.Errorf("%w: unknown kind %q", ErrBadSpec, kind)
	}
}

func NormalizeKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "mcp", "mcps":
		return KindMCP, nil
	case "plugin", "plugins":
		return KindPlugin, nil
	case "skill", "skills":
		return KindSkill, nil
	default:
		return "", fmt.Errorf("%w: unknown kind %q", ErrBadSpec, kind)
	}
}

func searchFields(kind string) string {
	switch kind {
	case KindMCP:
		return "id,name,description,isOfficial,license,repository,homepage,version,tags,transport,transports,requiredEnv"
	case KindPlugin:
		return "id,name,description,isOfficial,license,repository,homepage,version,tags,capabilities,platforms,arch"
	case KindSkill:
		return "name,skillId,source,isOfficial,url,license"
	default:
		return ""
	}
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("catalog: request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pegg")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog: %s returned %s", endpoint, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("catalog: read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("catalog: response exceeds %d bytes", maxResponseBytes)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("catalog: decode response: %w", err)
	}
	return nil
}

func ParseSkillRef(raw string) (string, string, error) {
	value := strings.Trim(strings.TrimSpace(raw), "/")
	if value == "" {
		return "", "", fmt.Errorf("%w: expected <owner>/<repo>/<skill>", ErrBadSpec)
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `\`) {
			return "", "", fmt.Errorf("%w: %q", ErrBadSpec, raw)
		}
	}
	switch len(parts) {
	case 2:
		return parts[0], parts[1], nil
	case 3:
		return parts[0] + "/" + parts[1], parts[2], nil
	default:
		return "", "", fmt.Errorf("%w: %q must be <owner>/<repo>/<skill> or <host>/<skill>", ErrBadSpec, raw)
	}
}

func ParseSkillSpec(raw string) (SkillSpec, error) {
	source, skillID, err := ParseSkillRef(raw)
	if err != nil {
		return SkillSpec{}, err
	}
	return SkillSpec{Source: source, SkillID: skillID}, nil
}

func escapeSegments(value string) string {
	parts := strings.Split(value, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
