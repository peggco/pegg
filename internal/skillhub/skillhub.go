package skillhub

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
	DefaultAPIBase  = "https://hub-api.pegg.dev/v1"
	DefaultGitHub   = "https://api.github.com"
	DefaultRawBase  = "https://raw.githubusercontent.com"
	maxSkillBytes   = 2 << 20
	defaultHTTPTime = 20 * time.Second
)

var (
	ErrBadSpec           = errors.New("skillhub: invalid skill reference")
	ErrSkillNotFound     = errors.New("skillhub: skill not found")
	ErrSourceUnsupported = errors.New("skillhub: source is not a GitHub repository")
)

type RemoteSkill struct {
	Name     string `json:"name"`
	SkillID  string `json:"skillId"`
	Source   string `json:"source"`
	URL      string `json:"url"`
	License  string `json:"license,omitempty"`
	Official bool   `json:"isOfficial"`
}

type Spec struct {
	Source  string
	SkillID string
}

func (s Spec) String() string { return s.Source + "/" + s.SkillID }

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
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PEGG_SKILLS_API")), "/")
	if base == "" {
		base = DefaultAPIBase
	}
	return &Client{
		BaseURL:   base,
		GitHubAPI: strings.TrimRight(DefaultGitHub, "/"),
		RawBase:   strings.TrimRight(DefaultRawBase, "/"),
		HTTP:      &http.Client{Timeout: defaultHTTPTime},
		Token:     strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		trees:     map[string][]treeEntry{},
	}
}

func ParseSpec(raw string) (Spec, error) {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "/")
	if s == "" {
		return Spec{}, fmt.Errorf("%w: expected <owner>/<repo>/<skill>", ErrBadSpec)
	}
	parts := strings.Split(s, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `\`) {
			return Spec{}, fmt.Errorf("%w: %q", ErrBadSpec, raw)
		}
	}
	switch len(parts) {
	case 2:
		return Spec{Source: parts[0], SkillID: parts[1]}, nil
	case 3:
		return Spec{Source: parts[0] + "/" + parts[1], SkillID: parts[2]}, nil
	default:
		return Spec{}, fmt.Errorf("%w: %q must be <owner>/<repo>/<skill> or <host>/<skill>", ErrBadSpec, raw)
	}
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

type SearchOptions struct {
	Query    string
	Limit    int
	Cursor   int
	Official *bool
}

type SearchResult struct {
	Items      []RemoteSkill
	NextCursor *int
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]RemoteSkill, error) {
	res, err := c.SearchPage(ctx, SearchOptions{Query: query, Limit: limit})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (c *Client) SearchPage(ctx context.Context, opts SearchOptions) (SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	u, err := url.Parse(c.BaseURL + "/skills")
	if err != nil {
		return SearchResult{}, fmt.Errorf("skillhub: build search url: %w", err)
	}
	q := u.Query()
	if query := strings.TrimSpace(opts.Query); query != "" {
		q.Set("q", query)
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("fields", "name,skillId,source,isOfficial,url,license")
	if opts.Cursor > 0 {
		q.Set("cursor", strconv.Itoa(opts.Cursor))
	}
	if opts.Official != nil {
		q.Set("official", strconv.FormatBool(*opts.Official))
	}
	u.RawQuery = q.Encode()

	var out struct {
		Items      []RemoteSkill `json:"items"`
		NextCursor *int          `json:"nextCursor"`
	}
	if err := c.getJSON(ctx, u.String(), &out); err != nil {
		return SearchResult{}, err
	}
	if out.Items == nil {
		out.Items = []RemoteSkill{}
	}
	return SearchResult{Items: out.Items, NextCursor: out.NextCursor}, nil
}

func (c *Client) Get(ctx context.Context, spec Spec) (RemoteSkill, error) {
	endpoint := c.BaseURL + "/skills/" + escapeSegments(spec.Source) + "/" + url.PathEscape(spec.SkillID)
	var out RemoteSkill
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return RemoteSkill{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("skillhub: request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pegg")
	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("skillhub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrSkillNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("skillhub: %s returned %s", endpoint, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSkillBytes))
	if err != nil {
		return fmt.Errorf("skillhub: read response: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("skillhub: decode response: %w", err)
	}
	return nil
}

func escapeSegments(source string) string {
	parts := strings.Split(source, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func isGitHubSource(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}
