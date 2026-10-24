package run

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/session"
)

func TestCrossProcessJournalAttachAndCancel(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sessions.db")
	log := logger.New(logger.LevelError)

	store1, err := session.NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store1.Close() })
	bus1 := event.New()
	mgr1 := session.NewManager(store1, bus1, log)
	rec1 := session.NewRecorder(mgr1, log)
	if err := rec1.Start(bus1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec1.Stop(bus1) })
	coord1 := New(mgr1, bus1, log)
	t.Cleanup(coord1.Stop)

	sess, err := mgr1.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	bus1.Publish(session.TopicSessionResume, session.SessionResume{AgentID: "agent-1", SessionID: sess.ID})

	lease, err := coord1.Claim(context.Background(), ClaimRequest{
		SessionID: sess.ID,
		AgentID:   "agent-1",
		AgentName: "orchestrator",
		Provider:  "p",
		Model:     "m",
	})
	if err != nil {
		t.Fatal(err)
	}

	bus1.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
	bus1.Publish(agent.TopicAgentInput, agent.AgentInput{AgentID: "agent-1", AgentName: "orchestrator", Input: "hello"})
	bus1.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "hi"})

	store2, err := session.NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	bus2 := event.New()
	mgr2 := session.NewManager(store2, bus2, log)
	coord2 := New(mgr2, bus2, log)
	t.Cleanup(coord2.Stop)

	if _, err := coord2.Claim(context.Background(), ClaimRequest{SessionID: sess.ID, AgentID: "agent-2"}); !errors.Is(err, session.ErrRunBusy) {
		t.Fatalf("second claim error = %v, want busy", err)
	}

	active, err := coord2.Active(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != lease.ID() {
		t.Fatalf("active run = %s, want %s", active.ID, lease.ID())
	}

	var events []session.RunEvent
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = coord2.Watch(context.Background(), active.ID, 0, func(e session.RunEvent) error {
			events = append(events, e)
			return nil
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		store2Events, _ := store2.RunEvents(active.ID, 0, 100)
		hasToken := false
		for _, e := range store2Events {
			if e.Type == session.AgentEventToken {
				hasToken = true
			}
		}
		if hasToken {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("token event was not journaled")
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := coord2.Cancel(active.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-lease.Context().Done():
	case <-time.After(3 * time.Second):
		t.Fatal("owner did not observe remote cancellation")
	}
	lease.Finish(session.RunStateCanceled, "canceled")

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not finish")
	}

	sawCanceled := false
	sawToken := false
	for _, e := range events {
		if e.Type == session.AgentEventToken {
			sawToken = true
		}
		if e.Type == session.RunEventState {
			sawCanceled = true
		}
	}
	if !sawToken || !sawCanceled {
		t.Fatalf("events = %+v, want token and final state", events)
	}
}
