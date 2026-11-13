package catalog

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestSearchMCPParamsAndProjection(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"id":"github","name":"GitHub","isOfficial":true,"transports":["stdio"]}],"nextCursor":3}`)
	}))
	defer srv.Close()

	official := true
	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	result, err := client.Search(context.Background(), "mcp", SearchOptions{
		Query:    "github",
		Limit:    5,
		Cursor:   2,
		Official: &official,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/mcp" {
		t.Fatalf("path = %q, want /mcp", gotPath)
	}
	if gotQuery.Get("q") != "github" || gotQuery.Get("limit") != "5" ||
		gotQuery.Get("cursor") != "2" || gotQuery.Get("official") != "true" {
		t.Fatalf("query = %v", gotQuery)
	}
	if got := gotQuery.Get("fields"); got != "id,name,description,isOfficial,license,repository,homepage,version,tags,transport,transports,requiredEnv" {
		t.Fatalf("fields = %q", got)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "github" || result.Items[0].Kind != KindMCP {
		t.Fatalf("items = %+v", result.Items)
	}
	if result.NextCursor == nil || *result.NextCursor != 3 {
		t.Fatalf("nextCursor = %v", result.NextCursor)
	}
}

func TestSearchPluginsParamsAndProjection(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = io.WriteString(w, `{"items":[{"id":"peggco/pegg-browser-use","name":"browser-use","platforms":["linux"]}]}`)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	result, err := client.Search(context.Background(), "plugins", SearchOptions{Query: "browser", Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/plugins" {
		t.Fatalf("path = %q, want /plugins", gotPath)
	}
	if gotQuery.Get("limit") != "200" {
		t.Fatalf("limit = %q, want capped 200", gotQuery.Get("limit"))
	}
	if got := gotQuery.Get("fields"); got != "id,name,description,isOfficial,license,repository,homepage,version,tags,capabilities,platforms,arch" {
		t.Fatalf("fields = %q", got)
	}
	if len(result.Items) != 1 || result.Items[0].Kind != KindPlugin {
		t.Fatalf("items = %+v", result.Items)
	}
}

func TestGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := client.Get(context.Background(), KindMCP, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEntriesPaths(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = io.WriteString(w, `{"id":"x","name":"X"}`)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := client.Get(context.Background(), KindMCP, "github"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(context.Background(), KindPlugin, "peggco/pegg-browser-use"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(context.Background(), KindSkill, "owner/repo/find-skills"); err != nil {
		t.Fatal(err)
	}
	want := []string{"/mcp/github", "/plugins/peggco/pegg-browser-use", "/skills/owner/repo/find-skills"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestConfigResolvesAllPlaceholderLocations(t *testing.T) {
	entry := Entry{
		Transport: Transport{
			Type:    "stdio",
			Command: "${BIN}",
			Args:    []string{"--token", "${TOKEN}"},
			Env:     map[string]string{"ENV_TOKEN": "${ENV_TOKEN}"},
		},
		RequiredEnv: []string{"BIN", "TOKEN", "ENV_TOKEN"},
	}
	cfg, missing, err := entry.Config(map[string]string{
		"BIN":       "npx",
		"TOKEN":     "token-value",
		"ENV_TOKEN": "env-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	if cfg.Command != "npx" || cfg.URL != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Args, []string{"--token", "token-value"}) {
		t.Fatalf("args = %v", cfg.Args)
	}
	if cfg.Env["ENV_TOKEN"] != "env-value" {
		t.Fatalf("env = %v", cfg.Env)
	}
}

func TestConfigResolvesRemoteURLAndHeaders(t *testing.T) {
	entry := Entry{
		Transport: Transport{
			Type:    "http",
			URL:     "https://${HOST}/mcp",
			Headers: map[string]string{"Authorization": "Bearer ${TOKEN}"},
		},
		RequiredEnv: []string{"HOST", "TOKEN"},
	}
	cfg, missing, err := entry.Config(map[string]string{"HOST": "example.com", "TOKEN": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	if cfg.URL != "https://example.com/mcp" {
		t.Fatalf("url = %q", cfg.URL)
	}
	if cfg.Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("headers = %v", cfg.Headers)
	}
	if cfg.Command != "" || len(cfg.Args) != 0 || len(cfg.Env) != 0 {
		t.Fatalf("remote config leaked stdio fields: %+v", cfg)
	}
}

func TestConfigUsesOSEnvAndSortsMissing(t *testing.T) {
	t.Setenv("TOKEN", "from-env")
	entry := Entry{
		Transport: Transport{
			Type:    "stdio",
			Command: "${BIN}",
			Args:    []string{"${ZED}"},
			Env:     map[string]string{"ALPHA": "${ALPHA}"},
		},
		RequiredEnv: []string{"BIN", "TOKEN", "ZED", "ALPHA"},
	}
	cfg, missing, err := entry.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Args[0] != "${ZED}" {
		t.Fatalf("unresolved args = %v", cfg.Args)
	}
	want := []string{"ALPHA", "BIN", "ZED"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
}

func TestConfigInfersTransportFromCommandOrURL(t *testing.T) {
	local := Entry{Transport: Transport{Command: "npx"}}
	cfg, _, err := local.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Command != "npx" || cfg.URL != "" {
		t.Fatalf("cfg = %+v", cfg)
	}

	remote := Entry{Transport: Transport{URL: "https://example.com/sse"}}
	cfg, _, err = remote.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "https://example.com/sse" || cfg.Command != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigRejectsInvalidTransport(t *testing.T) {
	cases := []Entry{
		{Transport: Transport{Type: "banana"}},
		{Transport: Transport{Type: "stdio"}},
		{Transport: Transport{Type: "sse"}},
	}
	for _, entry := range cases {
		if _, _, err := entry.Config(nil); err == nil {
			t.Fatalf("expected error for %+v", entry.Transport)
		}
	}
}

func TestSpecParsing(t *testing.T) {
	validMCP := []string{"github", "a", "a-1", strings.Repeat("a", 64)}
	for _, id := range validMCP {
		if _, err := ParseMCPID(id); err != nil {
			t.Fatalf("ParseMCPID(%q) = %v", id, err)
		}
	}
	invalidMCP := []string{"", "-a", "A", "a_b", "a/b", strings.Repeat("a", 65)}
	for _, id := range invalidMCP {
		if _, err := ParseMCPID(id); err == nil {
			t.Fatalf("ParseMCPID(%q) should fail", id)
		}
	}

	validPlugins := []string{"owner/repo", "Owner.Name/re_po-1", "a/b.c"}
	for _, id := range validPlugins {
		if _, err := ParsePluginID(id); err != nil {
			t.Fatalf("ParsePluginID(%q) = %v", id, err)
		}
	}
	invalidPlugins := []string{"", "owner", "owner/", "/repo", "owner/repo/extra", "../repo", "owner/.."}
	for _, id := range invalidPlugins {
		if _, err := ParsePluginID(id); err == nil {
			t.Fatalf("ParsePluginID(%q) should fail", id)
		}
	}

	source, skillID, err := ParseSkillRef("owner/repo/find-skills")
	if err != nil || source != "owner/repo" || skillID != "find-skills" {
		t.Fatalf("ParseSkillRef = %q, %q, %v", source, skillID, err)
	}
	if _, _, err := ParseSkillRef("owner"); err == nil {
		t.Fatal("ParseSkillRef(owner) should fail")
	}
}

func TestNewClientBaseURLPrecedence(t *testing.T) {
	t.Setenv("PEGG_CATALOG_API", "https://catalog.example/v1/")
	t.Setenv("PEGG_SKILLS_API", "https://skills.example/v1")
	if got := NewClient().BaseURL; got != "https://catalog.example/v1" {
		t.Fatalf("BaseURL = %q", got)
	}

	t.Setenv("PEGG_CATALOG_API", "")
	if got := NewClient().BaseURL; got != "https://skills.example/v1" {
		t.Fatalf("BaseURL = %q", got)
	}

	t.Setenv("PEGG_SKILLS_API", "")
	if got := NewClient().BaseURL; got != DefaultAPIBase {
		t.Fatalf("BaseURL = %q", got)
	}
}
