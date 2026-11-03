package searchskill

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/skillhub"
)

func testClient(server *httptest.Server) *skillhub.Client {
	c := skillhub.NewClient()
	c.BaseURL = server.URL
	c.GitHubAPI = server.URL
	c.RawBase = server.URL
	return c
}

func TestSearchSkillToolResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "docx" {
			t.Errorf("q = %q, want docx", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Errorf("limit = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"name":"docx","skillId":"docx","source":"anthropics/skills","url":"https://github.com/anthropics/skills?a=1&b=2","license":"MIT & Apache","isOfficial":true}]}`))
	}))
	defer server.Close()

	tt := searchSkillTool(testClient(server))
	out, err := tt.Execute(t.Context(), `{"query":"docx","maxResults":5}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<skill_results query="docx" count="1">`,
		"<name>docx</name>",
		"<skill_id>docx</skill_id>",
		"<source>anthropics/skills</source>",
		"<ref>anthropics/skills/docx</ref>",
		"<url>https://github.com/anthropics/skills?a=1&amp;b=2</url>",
		"<license>MIT &amp; Apache</license>",
		"<official>true</official>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSearchSkillToolEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	tt := searchSkillTool(testClient(server))
	out, err := tt.Execute(t.Context(), `{"query":"nothing"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `<skill_results query="nothing" count="0">`) {
		t.Fatalf("unexpected empty result:\n%s", out)
	}
}

func TestSearchSkillToolMissingQuery(t *testing.T) {
	tt := searchSkillTool(nil)
	if _, err := tt.Execute(t.Context(), `{}`); err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestSearchSkillToolServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	tt := searchSkillTool(testClient(server))
	if _, err := tt.Execute(t.Context(), `{"query":"docx"}`); err == nil {
		t.Fatal("expected error for server failure")
	}
}

func TestSearchSkillToolClampsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit = %q, want 20", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	tt := searchSkillTool(testClient(server))
	if _, err := tt.Execute(t.Context(), `{"query":"docx","maxResults":100}`); err != nil {
		t.Fatal(err)
	}
}

func TestEscapeXML(t *testing.T) {
	got := escapeXML(`<tag attr="a&b">'x'</tag>`)
	want := "&lt;tag attr=&quot;a&amp;b&quot;&gt;&apos;x&apos;&lt;/tag&gt;"
	if got != want {
		t.Fatalf("escapeXML = %q, want %q", got, want)
	}
}
