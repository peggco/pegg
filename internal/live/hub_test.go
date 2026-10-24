package live

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

func newLiveTest(t *testing.T) (*session.Manager, *runpkg.Coordinator, *Hub, event.Bus, *session.Session) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sessions.db")
	store, err := session.NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	log := logger.New(logger.LevelError)
	mgr := session.NewManager(store, bus, log)
	rec := session.NewRecorder(mgr, log)
	if err := rec.Start(bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Stop(bus) })
	coord := runpkg.New(mgr, bus, log)
	t.Cleanup(coord.Stop)
	hub := New(coord, mgr, bus, log)
	t.Cleanup(hub.Close)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(session.TopicSessionResume, session.SessionResume{AgentID: "agent-1", SessionID: sess.ID})
	return mgr, coord, hub, bus, sess
}

func TestHubDeliversShortRunAfterSubscription(t *testing.T) {
	_, coord, hub, bus, sess := newLiveTest(t)
	sub := hub.SubscribeSession(sess.ID, false, 0)
	t.Cleanup(sub.Close)

	lease, err := coord.Claim(context.Background(), runpkg.ClaimRequest{
		SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
	bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "hi"})
	lease.Finish(session.RunStateDone, "")

	var (
		sawStart  bool
		sawToken  bool
		sawFinish bool
	)
	deadline := time.After(3 * time.Second)
	for !sawStart || !sawToken || !sawFinish {
		select {
		case e := <-sub.C:
			switch e.Type {
			case EventRunStarted:
				sawStart = true
			case EventRunEvent:
				if e.Journal.Type == session.AgentEventToken {
					sawToken = true
				}
			case EventRunFinished:
				sawFinish = true
			}
		case <-deadline:
			t.Fatalf("timed out: start=%v token=%v finish=%v", sawStart, sawToken, sawFinish)
		}
	}
}

func TestHubSnapshotActiveRun(t *testing.T) {
	_, coord, hub, bus, sess := newLiveTest(t)
	lease, err := coord.Claim(context.Background(), runpkg.ClaimRequest{
		SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish(session.RunStateCanceled, "test")
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
	bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "snapshot"})
	time.Sleep(150 * time.Millisecond)

	events, _, err := hub.SnapshotSession(sess.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	var start, token bool
	for _, e := range events {
		if e.Type == EventRunStarted {
			start = true
		}
		if e.Type == EventRunEvent && e.Journal.Type == session.AgentEventToken {
			token = true
		}
	}
	if !start || !token {
		t.Fatalf("snapshot = %+v, want start and token", events)
	}
}

func TestHubFollowsSecondRunAfterFirstFinishes(t *testing.T) {
	_, coord, hub, bus, sess := newLiveTest(t)
	sub := hub.SubscribeSession(sess.ID, false, 0)
	t.Cleanup(sub.Close)

	run := func(tag string) {
		t.Helper()
		lease, err := coord.Claim(context.Background(), runpkg.ClaimRequest{
			SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
		})
		if err != nil {
			t.Fatal(err)
		}
		bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
		bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: tag})
		lease.Finish(session.RunStateDone, "")
	}
	run("one")
	waitFor := func(tag string) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case e := <-sub.C:
				if e.Type == EventRunEvent && e.Journal.Type == session.AgentEventToken {
					var d struct {
						Content string `json:"content"`
					}
					if json.Unmarshal(e.Journal.Data, &d) == nil && d.Content == tag {
						return
					}
				}
			case <-deadline:
				t.Fatalf("did not receive token %q", tag)
			}
		}
	}
	waitFor("one")
	run("two")
	waitFor("two")
}

func TestHubDoesNotDuplicateEvents(t *testing.T) {
	_, coord, hub, bus, sess := newLiveTest(t)
	sub := hub.SubscribeSession(sess.ID, false, 0)
	t.Cleanup(sub.Close)

	lease, err := coord.Claim(context.Background(), runpkg.ClaimRequest{
		SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
	bus.Publish(agent.TopicAgentInput, agent.AgentInput{AgentID: "agent-1", AgentName: "orchestrator", Input: "hello"})
	bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "hi"})
	lease.Finish(session.RunStateDone, "")

	counts := map[string]int{}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case e := <-sub.C:
			if e.Type == EventRunEvent {
				counts[e.Journal.Type]++
			}
			if e.Type == EventRunFinished {
				counts["finish"]++
				if counts[session.AgentEventInput] != 1 || counts[session.AgentEventToken] != 1 || counts["finish"] != 1 {
					t.Fatalf("duplicate events: %+v", counts)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out: %+v", counts)
		}
	}
}

func TestSnapshotThenSubscribeDoesNotDuplicate(t *testing.T) {
	_, coord, hub, bus, sess := newLiveTest(t)
	lease, err := coord.Claim(context.Background(), runpkg.ClaimRequest{
		SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish(session.RunStateCanceled, "test")
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{AgentID: "agent-1", AgentName: "orchestrator"})
	bus.Publish(agent.TopicAgentInput, agent.AgentInput{AgentID: "agent-1", AgentName: "orchestrator", Input: "hello"})
	bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "one"})
	time.Sleep(150 * time.Millisecond)

	snapshot, since, err := hub.SnapshotSession(sess.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	sub := hub.SubscribeSession(sess.ID, true, since)
	t.Cleanup(sub.Close)

	bus.Publish(agent.TopicAgentToken, agent.AgentToken{AgentID: "agent-1", AgentName: "orchestrator", Content: "two"})
	deadline := time.After(3 * time.Second)
	inputs, tokens := 0, 0
	for _, e := range snapshot {
		if e.Type == EventRunEvent {
			switch e.Journal.Type {
			case session.AgentEventInput:
				inputs++
			case session.AgentEventToken:
				tokens++
			}
		}
	}
	for {
		select {
		case e := <-sub.C:
			if e.Type == EventRunEvent {
				switch e.Journal.Type {
				case session.AgentEventInput:
					inputs++
				case session.AgentEventToken:
					tokens++
				}
			}
			if tokens >= 2 && inputs >= 1 {
				if tokens != 2 || inputs != 1 {
					t.Fatalf("duplicates: inputs=%d tokens=%d", inputs, tokens)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out: inputs=%d tokens=%d", inputs, tokens)
		}
	}
}
