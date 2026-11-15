package session

import (
	"context"
	"testing"

	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/llm"
)

func withHooks(t *testing.T) {
	t.Helper()
	corehook.SetDefault(corehook.NewBus())
	t.Cleanup(corehook.ResetDefault)
}

func TestCreateHookRewritesTitle(t *testing.T) {
	withHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookCreate, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		if payload["title"] == "original" {
			payload["title"] = "renamed"
		}
		return payload, nil
	})
	mgr, _ := newTestManager(t)
	s, err := mgr.Create(CreateOptions{Title: "original", Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "renamed" {
		t.Fatalf("title = %q", s.Title)
	}
}

func TestMessageAppendHookRewritesContent(t *testing.T) {
	withHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookMessageAppend, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		if msg, ok := payload["message"].(map[string]any); ok {
			msg["content"] = "hooked"
		}
		return payload, nil
	})
	mgr, _ := newTestManager(t)
	s, err := mgr.Create(CreateOptions{Title: "s", Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(s.ID, llm.UserMessage("original")); err != nil {
		t.Fatal(err)
	}
	msgs, err := mgr.Messages(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d", len(msgs))
	}
	if got := llm.MessageText(llm.Message{Content: msgs[0].Content}); got != "hooked" {
		t.Fatalf("content = %q", got)
	}
}

func TestTitleHookRewritesTitle(t *testing.T) {
	withHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookTitle, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["title"] = "[tag] " + payload["title"].(string)
		return payload, nil
	})
	mgr, _ := newTestManager(t)
	s, err := mgr.Create(CreateOptions{Title: "s", Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetTitle(s.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Get(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "[tag] renamed" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestForkAndDeleteHooks(t *testing.T) {
	withHooks(t)
	var forked, deleted int
	corehook.Default().Registry().AddActionFor("test", HookFork, 10, func(ctx context.Context, value any) (any, error) {
		forked++
		return nil, nil
	})
	corehook.Default().Registry().AddActionFor("test", HookDelete, 10, func(ctx context.Context, value any) (any, error) {
		deleted++
		return nil, nil
	})
	mgr, _ := newTestManager(t)
	s, ids := newSessionWithMessages(t, mgr, "s", 2)
	if _, err := mgr.Fork(s.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	if forked != 1 {
		t.Fatalf("fork hooks = %d", forked)
	}
	if err := mgr.Delete(s.ID); err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("delete hooks = %d", deleted)
	}
}

func TestCompactHookRewritesMessages(t *testing.T) {
	withHooks(t)
	corehook.Default().Registry().AddFilterFor("test", HookCompact, 10, func(ctx context.Context, value any) (any, error) {
		payload := value.(map[string]any)
		payload["messages"] = []any{map[string]any{"role": "user", "content": "summarized"}}
		return payload, nil
	})
	mgr, _ := newTestManager(t)
	s, _ := newSessionWithMessages(t, mgr, "s", 2)
	compacted, err := mgr.CompactSession(s.ID, []llm.Message{llm.UserMessage("old")}, "test")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := mgr.Messages(compacted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d", len(msgs))
	}
	if got := llm.MessageText(llm.Message{Content: msgs[0].Content}); got != "summarized" {
		t.Fatalf("content = %q", got)
	}
}
