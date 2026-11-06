package acp

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

func newHistoryServer(t *testing.T) (*Server, *session.Manager, *runpkg.Coordinator) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStoreAt(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	bus := event.New()
	log := logger.New(logger.LevelError)
	mgr := session.NewManager(store, bus, log)
	coord := runpkg.New(mgr, bus, log)
	t.Cleanup(coord.Stop)
	hub := live.New(coord, mgr, bus, log)
	t.Cleanup(hub.Close)

	srv := New(config.DefaultConfig(), bus, log, nil, mgr, nil, coord, hub)
	return srv, mgr, coord
}

func TestSessionHistoryFiltersFirstRunMessage(t *testing.T) {
	srv, mgr, _ := newHistoryServer(t)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.UserMessage("hello")); err != nil {
		t.Fatal(err)
	}

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "first-run", SessionID: sess.ID, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", BaseSeq: 0,
		StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}

	msgs, err := srv.sessionHistory(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("history = %d messages, want 0 (active-run message is replayed from the journal)", len(msgs))
	}
}

func TestSessionHistoryKeepsMessagesUpToBaseSeq(t *testing.T) {
	srv, mgr, _ := newHistoryServer(t)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []llm.Message{
		llm.UserMessage("first"),
		llm.AssistantMessage("answer one"),
		llm.UserMessage("second"),
		llm.AssistantMessage("stale"),
	} {
		if _, err := mgr.AppendMessage(sess.ID, m); err != nil {
			t.Fatal(err)
		}
	}

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "second-run", SessionID: sess.ID, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", BaseSeq: 2,
		StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}

	msgs, err := srv.sessionHistory(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("history = %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "first" || msgs[1].Content != "answer one" {
		t.Fatalf("history = %+v", msgs)
	}
}
