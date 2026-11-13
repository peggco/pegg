package install

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultAPIBase       = "https://api.github.com"
	defaultAPITimeout    = 60 * time.Second
	defaultAPIMaxBytes   = 4 << 20
	defaultDownloadBytes = 200 << 20
)

type Asset struct {
	Name   string
	URL    string
	Size   int64
	Digest string
}

type Release struct {
	TagName string
	Name    string
	Assets  []Asset
}

type Client struct {
	APIBase       string
	HTTP          *http.Client
	Token         string
	UserAgent     string
	MaxAPIBytes   int64
	MaxAssetBytes int64
}

func NewClient() *Client {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	api := strings.TrimRight(strings.TrimSpace(os.Getenv("PEGG_GITHUB_API")), "/")
	if api == "" {
		api = DefaultAPIBase
	}
	return &Client{
		APIBase:       api,
		HTTP:          &http.Client{Timeout: defaultAPITimeout},
		Token:         token,
		UserAgent:     "pegg",
		MaxAPIBytes:   defaultAPIMaxBytes,
		MaxAssetBytes: defaultDownloadBytes,
	}
}

func (c *Client) Latest(ctx context.Context, source Source) (*Release, error) {
	return c.release(ctx, fmt.Sprintf("/repos/%s/%s/releases/latest", source.Owner, source.Repo), source)
}

func (c *Client) ByTag(ctx context.Context, source Source, tag string) (*Release, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return c.Latest(ctx, source)
	}
	return c.release(ctx, fmt.Sprintf("/repos/%s/%s/releases/tags/%s", source.Owner, source.Repo, url.PathEscape(tag)), source)
}

func (c *Client) release(ctx context.Context, path string, source Source) (*Release, error) {
	body, status, header, err := c.get(ctx, c.APIBase+path)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("release not found for %s", source.Slug())
	}
	if status < 200 || status >= 300 {
		return nil, apiError(status, header, body, source)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode release response: %w", err)
	}
	release := &Release{TagName: payload.TagName, Name: payload.Name}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{
			Name:   asset.Name,
			URL:    asset.BrowserDownloadURL,
			Size:   asset.Size,
			Digest: asset.Digest,
		})
	}
	return release, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("request GitHub API: %w", err)
	}
	defer resp.Body.Close()
	limit := c.MaxAPIBytes
	if limit <= 0 {
		limit = defaultAPIMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("read GitHub API response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("GitHub API response exceeds %d bytes", limit)
	}
	return body, resp.StatusCode, resp.Header, nil
}

func (c *Client) Download(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = c.MaxAssetBytes
	}
	if maxBytes <= 0 {
		maxBytes = defaultDownloadBytes
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("download release asset: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read release asset: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("release asset exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultAPITimeout}
}

func apiError(status int, header http.Header, body []byte, source Source) error {
	message := strings.TrimSpace(string(body))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Message != "" {
		message = payload.Message
	}
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		if header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit reached for %s; set GITHUB_TOKEN: %s", source.Slug(), message)
		}
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("GitHub API error %d for %s: %s", status, source.Slug(), message)
}
