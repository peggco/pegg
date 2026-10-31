package acp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

func TestSessionNewFollowsRunsFromOtherClients(t *testing.T) {
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
	tr := newBufferTransport()
	srv.transport = tr
	srv.newAgentFn = mockAgentFn()

	result, err := srv.handleSessionNew(context.Background(), toJSON(t, SessionNewParams{Cwd: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	sr := result.(SessionNewResult)
	if sr.PeggSessionId == "" {
		t.Fatal("no pegg session id")
	}

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "remote-run", SessionID: sr.PeggSessionId, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run.ID, SessionID: run.SessionID, Type: session.RunEventStarted, AgentID: run.AgentID, Data: []byte(`{"base_seq":0}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventInput, AgentID: run.AgentID, Data: []byte(`{"input":"hello from tui"}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventToken, AgentID: run.AgentID, Data: []byte(`{"content":"reply"}`)},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		tr.mu.Lock()
		all := strings.Join(tr.sends, "\n")
		tr.mu.Unlock()
		if strings.Contains(all, "hello from tui") && strings.Contains(all, "reply") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("notifications did not include remote run: %s", all)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestResolveAskCancelsPendingAndNotifiesClient(t *testing.T) {
	srv := &Server{active: make(map[string]*ACPSession), agents: make(map[string]string), pendingAsks: make(map[string]*pendingAsk)}
	tr := newBufferTransport()
	srv.transport = tr

	ctx, cancel := context.WithCancel(context.Background())
	p := srv.registerAsk("run:r1", "acp-1", cancel)
	srv.handleRunAskResolved(session.RunAskResolved{RunID: "r1", AgentID: "a1"})

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("pending ask context was not cancelled")
	}
	if !p.resolved.Load() {
		t.Fatal("pending ask was not marked resolved")
	}
	if !strings.Contains(tr.last(), "run_ask_resolved") {
		t.Fatalf("notification = %s", tr.last())
	}
}

func TestSessionUpdatedPushesModeToAcpSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStoreAt(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	log := logger.New(logger.LevelError)
	mgr := session.NewManager(store, bus, log)
	sess, err := mgr.Create(session.CreateOptions{Mode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	orch := newMustAgent(t)
	acpSess := &ACPSession{ID: "acp-1", Agent: orch, PeggSessionID: sess.ID}
	srv := &Server{sessions: mgr, active: map[string]*ACPSession{acpSess.ID: acpSess}, agents: map[string]string{}, pendingAsks: make(map[string]*pendingAsk)}
	tr := newBufferTransport()
	srv.transport = tr

	if err := mgr.SetMode(sess.ID, "plan"); err != nil {
		t.Fatal(err)
	}
	srv.handleSessionUpdated(session.SessionUpdated{SessionID: sess.ID})
	if got := orch.CurrentMode(); got != "plan" {
		t.Fatalf("mode = %q, want plan", got)
	}
	if !strings.Contains(tr.last(), "current_mode_update") || !strings.Contains(tr.last(), "plan") {
		t.Fatalf("notification = %s", tr.last())
	}
}

func newMustAgent(t *testing.T) *agent.Agent {
	t.Helper()
	return agent.New("test-agent")
}

func TestFollowEmitsToolCallAndCompletion(t *testing.T) {
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
	tr := newBufferTransport()
	srv.transport = tr
	srv.newAgentFn = mockAgentFn()

	result, err := srv.handleSessionNew(context.Background(), toJSON(t, SessionNewParams{Cwd: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	sid := result.(SessionNewResult).PeggSessionId

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "remote-tools", SessionID: sid, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run.ID, SessionID: run.SessionID, Type: session.RunEventStarted, AgentID: run.AgentID, Data: []byte(`{"base_seq":0}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventToolCall, AgentID: run.AgentID, Data: []byte(`{"call_id":"call-1","tool_name":"bash","args":"{}","call":{"id":"call-1","type":"function","function":{"name":"bash","arguments":"{}"}}}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventToolResult, AgentID: run.AgentID, Data: []byte(`{"call_id":"call-1","tool_name":"bash","output":"ok","error":""}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.RunEventState, AgentID: run.AgentID, Data: []byte(`{"state":"done"}`)},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		tr.mu.Lock()
		all := strings.Join(tr.sends, "\n")
		tr.mu.Unlock()
		if strings.Contains(all, `"sessionUpdate":"tool_call"`) &&
			strings.Contains(all, `"toolCallId":"call-1"`) &&
			strings.Contains(all, `"sessionUpdate":"tool_call_update"`) &&
			strings.Contains(all, `"status":"completed"`) &&
			strings.Contains(all, `"runState":"done"`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("follow did not emit tool updates: %s", all)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFollowSettlesUnfinishedTools(t *testing.T) {
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
	tr := newBufferTransport()
	srv.transport = tr
	srv.newAgentFn = mockAgentFn()

	result, err := srv.handleSessionNew(context.Background(), toJSON(t, SessionNewParams{Cwd: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	sid := result.(SessionNewResult).PeggSessionId

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "remote-stuck", SessionID: sid, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run.ID, SessionID: run.SessionID, Type: session.RunEventStarted, AgentID: run.AgentID, Data: []byte(`{"base_seq":0}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventToolCall, AgentID: run.AgentID, Data: []byte(`{"call_id":"lost-result","tool_name":"edit","args":"{}","call":{"id":"lost-result","type":"function","function":{"name":"edit","arguments":"{}"}}}`)},
		{RunID: run.ID, SessionID: run.SessionID, Type: session.AgentEventMessage, AgentID: run.AgentID, Data: []byte(`{"message":{"role":"assistant","content":"done"}}`)},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		tr.mu.Lock()
		all := strings.Join(tr.sends, "\n")
		tr.mu.Unlock()
		if strings.Contains(all, `"toolCallId":"lost-result"`) && strings.Contains(all, `"status":"completed"`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("unfinished tool was not settled: %s", all)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
