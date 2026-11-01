package skillhub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSkillMD = "---\nname: grill-me\ndescription: Grills you about a plan\nlicense: MIT\n---\n\nAsk hard questions.\n"

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/skills", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "grill" {
			t.Errorf("q = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"name":"grill-me","skillId":"grill-me","source":"mattpocock/skills","url":"https://github.com/mattpocock/skills","license":"MIT","isOfficial":false}],"nextCursor":null}`))
	})
	mux.HandleFunc("/repos/mattpocock/skills/git/trees/HEAD", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tree":[{"path":"skills/engineering/grill-me/SKILL.md","type":"blob"},{"path":"README.md","type":"blob"}]}`))
	})
	mux.HandleFunc("/mattpocock/skills/HEAD/skills/engineering/grill-me/SKILL.md", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testSkillMD))
	})
	mux.HandleFunc("/mattpocock/skills/HEAD/README.md", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# Skills\n\nHello."))
	})
	return httptest.NewServer(mux)
}

func testClient(server *httptest.Server) *Client {
	c := NewClient()
	c.BaseURL = server.URL
	c.GitHubAPI = server.URL
	c.RawBase = server.URL
	return c
}

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in     string
		source string
		skill  string
		ok     bool
	}{
		{"mattpocock/skills/grill-me", "mattpocock/skills", "grill-me", true},
		{"https://example.com/vercel-labs/skills/find-skills", "", "", false},
		{"open.feishu.cn/foo", "open.feishu.cn", "foo", true},
		{"", "", "", false},
		{"a/b/c/d", "", "", false},
		{"a/../c", "", "", false},
	}
	for _, tc := range cases {
		got, err := ParseSpec(tc.in)
		if tc.ok {
			if err != nil {
				t.Fatalf("ParseSpec(%q) error: %v", tc.in, err)
			}
			if got.Source != tc.source || got.SkillID != tc.skill {
				t.Fatalf("ParseSpec(%q) = %+v", tc.in, got)
			}
			continue
		}
		if err == nil {
			t.Fatalf("ParseSpec(%q) expected error", tc.in)
		}
	}
}

func TestSearch(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	items, err := testClient(server).Search(context.Background(), "grill", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SkillID != "grill-me" || items[0].Source != "mattpocock/skills" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].URL != "https://github.com/mattpocock/skills" {
		t.Fatalf("url = %q", items[0].URL)
	}
}

func TestResolve(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	data, path, err := testClient(server).Resolve(context.Background(), Spec{Source: "mattpocock/skills", SkillID: "grill-me"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "skills/engineering/grill-me/SKILL.md" {
		t.Fatalf("path = %q", path)
	}
	if !strings.Contains(string(data), "Ask hard questions") {
		t.Fatalf("data = %q", data)
	}
}

func TestReadme(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	text, err := testClient(server).Readme(context.Background(), "mattpocock/skills")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "# Skills") {
		t.Fatalf("readme = %q", text)
	}
	if _, err := testClient(server).Readme(context.Background(), "open.feishu.cn"); err == nil {
		t.Fatal("expected error for non-GitHub source")
	}
}

func TestInstallListRemove(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	client := testClient(server)
	dir := t.TempDir()

	installed, err := Install(context.Background(), client, Spec{Source: "mattpocock/skills", SkillID: "grill-me"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Name != "grill-me" || installed.Source != "mattpocock/skills" {
		t.Fatalf("installed = %+v", installed)
	}
	if _, err := os.Stat(filepath.Join(dir, "grill-me", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	list := ListInstalled(dir, "global")
	if len(list) != 1 || list[0].Name != "grill-me" || list[0].Scope != "global" {
		t.Fatalf("list = %+v", list)
	}

	if err := Remove(dir, "grill-me"); err != nil {
		t.Fatal(err)
	}
	if list := ListInstalled(dir, "global"); len(list) != 0 {
		t.Fatalf("list after remove = %+v", list)
	}
	if err := Remove(dir, "grill-me"); err == nil {
		t.Fatal("expected error removing missing skill")
	}
}
