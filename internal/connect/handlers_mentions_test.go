package connect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/vfs"
)

func mentionsResult(t *testing.T, result any) []mentionItem {
	t.Helper()
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	items, ok := m["items"].([]mentionItem)
	if !ok {
		t.Fatalf("items type = %T, want []mentionItem", m["items"])
	}
	return items
}

func TestMentionsListWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.go"), []byte("package src"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}

	c, err := New(Deps{Config: config.DefaultConfig(), Log: testLogger(), Version: "test", VFS: fs}, config.DefaultConfig().Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	result, err := c.handleMentionsList(context.Background(), &Request{})
	if err != nil {
		t.Fatal(err)
	}

	kinds := map[string]string{}
	for _, it := range mentionsResult(t, result) {
		kinds[it.Label] = it.Kind
	}
	if kinds["main.go"] != "file" {
		t.Fatalf("main.go kind = %q, want file", kinds["main.go"])
	}
	if kinds["src/app.go"] != "file" {
		t.Fatalf("src/app.go kind = %q, want file", kinds["src/app.go"])
	}
	if kinds["src"] != "folder" {
		t.Fatalf("src kind = %q, want folder", kinds["src"])
	}
}

func TestMentionsListAgents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := agents.Register(func() (*agent.Agent, error) {
		return agent.New("tester", agent.WithDescription("does tests")), nil
	}); err != nil {
		t.Fatal(err)
	}

	c, err := New(Deps{Config: config.DefaultConfig(), Log: testLogger(), Version: "test"}, config.DefaultConfig().Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	result, err := c.handleMentionsList(context.Background(), &Request{})
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, it := range mentionsResult(t, result) {
		if it.Label != "tester" {
			continue
		}
		found = true
		if it.Kind != "agent" {
			t.Fatalf("tester kind = %q, want agent", it.Kind)
		}
		if it.Detail != "does tests" {
			t.Fatalf("tester detail = %q, want %q", it.Detail, "does tests")
		}
	}
	if !found {
		t.Fatal("registered agent missing from mentions")
	}
}

func TestMentionsListQueryAndLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	root := t.TempDir()
	for _, name := range []string{"alpha.go", "beta.go", "gamma.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}

	c, err := New(Deps{Config: config.DefaultConfig(), Log: testLogger(), Version: "test", VFS: fs}, config.DefaultConfig().Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	filtered, err := c.handleMentionsList(context.Background(), &Request{Params: []byte(`{"query":"alpha"}`)})
	if err != nil {
		t.Fatal(err)
	}
	items := mentionsResult(t, filtered)
	if len(items) != 1 || items[0].Label != "alpha.go" {
		t.Fatalf("query filter = %+v, want only alpha.go", items)
	}

	limited, err := c.handleMentionsList(context.Background(), &Request{Params: []byte(`{"query":".go","limit":2}`)})
	if err != nil {
		t.Fatal(err)
	}
	m := limited.(map[string]any)
	if got := len(m["items"].([]mentionItem)); got != 2 {
		t.Fatalf("limit = %d items, want 2", got)
	}
	if m["truncated"] != true {
		t.Fatalf("truncated = %v, want true", m["truncated"])
	}
}
