package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

func TestActivateSessionAttachesToRemoteRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	mgr := session.NewManager(store, bus, logger.New(logger.LevelError))
	coord := runpkg.New(mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(coord.Stop)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleUser, Content: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleAssistant, Content: "answer"}); err != nil {
		t.Fatal(err)
	}
	base := 2
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleUser, Content: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleAssistant, Content: "stale"}); err != nil {
		t.Fatal(err)
	}

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	remote := session.Run{
		ID:            "remote-run",
		SessionID:     sess.ID,
		State:         session.RunStateRunning,
		OwnerInstance: "other-process",
		AgentID:       "remote-agent",
		BaseSeq:       base,
		StartedAt:     now,
		UpdatedAt:     now,
	}
	if err := rs.CreateRun(remote); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: remote.ID, SessionID: sess.ID, Type: session.AgentEventInput, AgentID: remote.AgentID,
			Data: []byte(`{"input":"second"}`)},
		{RunID: remote.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: remote.AgentID,
			Data: []byte(`{"content":"live token"}`)},
	}); err != nil {
		t.Fatal(err)
	}

	orch := agent.New("orchestrator")
	hub := live.New(coord, mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(hub.Close)
	app := &App{
		bus:         bus,
		deps:        settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent:       orch,
		chat:        components.NewChat(),
		ctx:         context.Background(),
		subs:        map[string]*agentTranscript{},
		subItemByID: map[string]*components.ChatItem{},
		mode:        agent.ModeAuto,
	}
	t.Cleanup(app.stopAttach)

	msgs, err := mgr.Messages(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	app.activateSession(settings.SessionInfo{ID: sess.ID, Title: "S", Provider: "p", Model: "m", Messages: msgs})

	deadline := time.Now().Add(3 * time.Second)
	for {
		app.chatMu.Lock()
		running, runID := app.running, app.attachRunID
		app.chatMu.Unlock()
		if running && runID == remote.ID {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("attach state running=%v run=%q, want true/%q", running, runID, remote.ID)
		}
		time.Sleep(10 * time.Millisecond)
	}

	for {
		text := ""
		if app.main != nil {
			for _, it := range app.main.items {
				if it.Kind == components.ItemAssistant {
					text += it.Text
				}
			}
		}
		if text == "answerlive token" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("attached transcript = %q, want live token", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, it := range app.main.items {
		if it.Kind == components.ItemAssistant && it.Text == "stale" {
			t.Fatal("stale in-flight message was not filtered")
		}
	}

	if err := rs.AppendRunEvents([]session.RunEvent{{
		RunID: remote.ID, SessionID: sess.ID, Type: session.RunEventState,
		AgentID: remote.AgentID, Data: []byte(`{"state":"done"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := rs.FinishRun(remote.ID, session.RunStateDone, ""); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		app.chatMu.Lock()
		running := app.running
		app.chatMu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("attached run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}

	now = time.Now()
	second := remote
	second.ID = "remote-run-2"
	second.StartedAt = now
	second.UpdatedAt = now
	if err := rs.CreateRun(second); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: second.ID, SessionID: sess.ID, Type: session.AgentEventInput, AgentID: second.AgentID,
			Data: []byte(`{"input":"third"}`)},
		{RunID: second.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: second.AgentID,
			Data: []byte(`{"content":"second live"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		text, _ := transcriptText(app)
		if strings.Contains(text, "second live") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("second run was not streamed: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func transcriptText(a *App) (string, []*components.ChatItem) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	var items []*components.ChatItem
	if a.main != nil {
		items = append(items, a.main.items...)
	}
	text := ""
	for _, it := range items {
		if it.Kind == components.ItemAssistant {
			text += it.Text
		}
	}
	return text, items
}

func TestFollowStreamsRunStartedAfterSessionOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	mgr := session.NewManager(store, bus, logger.New(logger.LevelError))
	coord := runpkg.New(mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(coord.Stop)
	hub := live.New(coord, mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(hub.Close)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	orch := agent.New("orchestrator")
	app := &App{
		bus:         bus,
		deps:        settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent:       orch,
		chat:        components.NewChat(),
		ctx:         context.Background(),
		subs:        map[string]*agentTranscript{},
		subItemByID: map[string]*components.ChatItem{},
		mode:        agent.ModeAuto,
	}
	t.Cleanup(app.stopAttach)
	app.activateSession(settings.SessionInfo{ID: sess.ID, Title: "s", Provider: "p", Model: "m"})

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "late-run", SessionID: sess.ID, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run.ID, SessionID: sess.ID, Type: session.AgentEventInput, AgentID: run.AgentID,
			Data: []byte(`{"input":"late"}`)},
		{RunID: run.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: run.AgentID,
			Data: []byte(`{"content":"answer"}`)},
		{RunID: run.ID, SessionID: sess.ID, Type: session.AgentEventMessage, AgentID: run.AgentID,
			Data: []byte(`{"message":{"role":"assistant","content":"answer"}}`)},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		text, items := transcriptText(app)
		if strings.Contains(text, "answer") {
			users, assistants := 0, 0
			for _, it := range items {
				if it.Kind == components.ItemUser && it.Text == "late" {
					users++
				}
				if it.Kind == components.ItemAssistant && strings.Contains(it.Text, "answer") {
					assistants++
				}
			}
			if users != 1 || assistants != 1 {
				t.Fatalf("duplicate live items: users=%d assistants=%d", users, assistants)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("late run was not streamed: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFollowDoesNotDuplicateWhenNextRunStartsImmediately(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	mgr := session.NewManager(store, bus, logger.New(logger.LevelError))
	coord := runpkg.New(mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(coord.Stop)
	hub := live.New(coord, mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(hub.Close)

	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run1 := session.Run{ID: "run-1", SessionID: sess.ID, State: session.RunStateRunning, OwnerInstance: "other", AgentID: "agent-x", StartedAt: now, UpdatedAt: now}
	if err := rs.CreateRun(run1); err != nil {
		t.Fatal(err)
	}
	_ = rs.AppendRunEvents([]session.RunEvent{
		{RunID: run1.ID, SessionID: sess.ID, Type: session.AgentEventInput, AgentID: run1.AgentID, Data: []byte(`{"input":"first"}`)},
		{RunID: run1.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: run1.AgentID, Data: []byte(`{"content":"one"}`)},
	})

	orch := agent.New("orchestrator")
	app := &App{
		bus: bus, deps: settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent: orch, chat: components.NewChat(), ctx: context.Background(),
		subs: map[string]*agentTranscript{}, subItemByID: map[string]*components.ChatItem{}, mode: agent.ModeAuto,
	}
	t.Cleanup(app.stopAttach)
	msgs, _ := mgr.Messages(sess.ID)
	app.activateSession(settings.SessionInfo{ID: sess.ID, Title: "s", Provider: "p", Model: "m", Messages: msgs})

	deadline := time.Now().Add(3 * time.Second)
	for {
		text, _ := transcriptText(app)
		if strings.Contains(text, "one") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("first run not attached: %q", text)
		}
		time.Sleep(10 * time.Millisecond)
	}

	run2 := session.Run{ID: "run-2", SessionID: sess.ID, State: session.RunStateRunning, OwnerInstance: "other", AgentID: "agent-x", StartedAt: time.Now(), UpdatedAt: time.Now()}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run1.ID, SessionID: sess.ID, Type: session.RunEventState, AgentID: run1.AgentID, Data: []byte(`{"state":"done"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := rs.FinishRun(run1.ID, session.RunStateDone, ""); err != nil {
		t.Fatal(err)
	}
	if err := rs.CreateRun(run2); err != nil {
		t.Fatal(err)
	}
	if err := rs.AppendRunEvents([]session.RunEvent{
		{RunID: run2.ID, SessionID: sess.ID, Type: session.RunEventStarted, AgentID: run2.AgentID, Data: []byte(`{"base_seq":0}`)},
		{RunID: run2.ID, SessionID: sess.ID, Type: session.AgentEventInput, AgentID: run2.AgentID, Data: []byte(`{"input":"second question"}`)},
		{RunID: run2.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: run2.AgentID, Data: []byte(`{"content":"two"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleUser, Content: "second question"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{Role: llm.RoleAssistant, Content: "two"}); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for {
		text, items := transcriptText(app)
		if strings.Contains(text, "one") && strings.Contains(text, "two") {
			users, ones, twos := 0, 0, 0
			for _, it := range items {
				switch {
				case it.Kind == components.ItemUser && it.Text == "first":
					users++
				case it.Kind == components.ItemUser && it.Text == "second question":
					users++
				case it.Kind == components.ItemAssistant && it.Text == "one":
					ones++
				case it.Kind == components.ItemAssistant && it.Text == "two":
					twos++
				}
			}
			if users != 2 || ones != 1 || twos != 1 {
				for _, it := range items {
					t.Logf("item kind=%d text=%q", it.Kind, it.Text)
				}
				t.Fatalf("duplicate or missing items: users=%d one=%d two=%d (%q)", users, ones, twos, text)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("second run not streamed: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTwoTuiFollowersKeepWorkingAfterLocalRuns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	log := logger.New(logger.LevelError)

	store1, err := session.NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store1.Close() })
	bus1 := event.New()
	mgr1 := session.NewManager(store1, bus1, log)
	coord1 := runpkg.New(mgr1, bus1, log)
	t.Cleanup(coord1.Stop)
	hub1 := live.New(coord1, mgr1, bus1, log)
	t.Cleanup(hub1.Close)

	store2, err := session.NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store2.Close() })
	bus2 := event.New()
	mgr2 := session.NewManager(store2, bus2, log)
	coord2 := runpkg.New(mgr2, bus2, log)
	t.Cleanup(coord2.Stop)
	hub2 := live.New(coord2, mgr2, bus2, log)
	t.Cleanup(hub2.Close)

	sess, err := mgr1.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	newApp := func(mgr *session.Manager, coord *runpkg.Coordinator, hub *live.Hub, bus event.Bus) *App {
		orch := agent.New("orchestrator")
		a := &App{
			bus: bus, deps: settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
			agent: orch, chat: components.NewChat(), ctx: context.Background(),
			subs: map[string]*agentTranscript{}, subItemByID: map[string]*components.ChatItem{}, mode: agent.ModeAuto,
		}
		t.Cleanup(a.stopAttach)
		msgs, _ := mgr.Messages(sess.ID)
		a.activateSession(settings.SessionInfo{ID: sess.ID, Title: "s", Provider: "p", Model: "m", Messages: msgs})
		return a
	}
	app1 := newApp(mgr1, coord1, hub1, bus1)
	app2 := newApp(mgr2, coord2, hub2, bus2)
	_ = app2
	time.Sleep(100 * time.Millisecond)

	localRun := func(n int) {
		t.Helper()
		lease, err := coord1.Claim(context.Background(), runpkg.ClaimRequest{
			SessionID: sess.ID, AgentID: "agent-1", AgentName: "orchestrator", Provider: "p", Model: "m",
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = store1.AppendRunEvents([]session.RunEvent{
			{RunID: lease.ID(), SessionID: sess.ID, Type: session.AgentEventInput, AgentID: "agent-1", Data: []byte(`{"input":"local-` + string(rune('0'+n)) + `"}`)},
			{RunID: lease.ID(), SessionID: sess.ID, Type: session.AgentEventToken, AgentID: "agent-1", Data: []byte(`{"content":"local-` + string(rune('0'+n)) + `"}`)},
		})
		lease.Finish(session.RunStateDone, "")
	}
	localRun(1)
	localRun(2)

	lease, err := coord2.Claim(context.Background(), runpkg.ClaimRequest{
		SessionID: sess.ID, AgentID: "agent-2", AgentName: "orchestrator", Provider: "p", Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store2.AppendRunEvents([]session.RunEvent{
		{RunID: lease.ID(), SessionID: sess.ID, Type: session.AgentEventInput, AgentID: "agent-2", Data: []byte(`{"input":"from-second"}`)},
		{RunID: lease.ID(), SessionID: sess.ID, Type: session.AgentEventToken, AgentID: "agent-2", Data: []byte(`{"content":"second-reply"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	defer lease.Finish(session.RunStateCanceled, "test")

	deadline := time.Now().Add(3 * time.Second)
	for {
		text, _ := transcriptText(app1)
		if strings.Contains(text, "second-reply") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("first TUI did not receive second TUI's run: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestEnsureFollowStartsForNewlyCreatedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bus := event.New()
	mgr := session.NewManager(store, bus, logger.New(logger.LevelError))
	coord := runpkg.New(mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(coord.Stop)
	hub := live.New(coord, mgr, bus, logger.New(logger.LevelError))
	t.Cleanup(hub.Close)
	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	orch := agent.New("orchestrator")
	app := &App{
		bus: bus, deps: settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent: orch, chat: components.NewChat(), ctx: context.Background(),
		subs: map[string]*agentTranscript{}, subItemByID: map[string]*components.ChatItem{}, mode: agent.ModeAuto,
	}
	t.Cleanup(app.stopAttach)
	app.ensureFollow(sess.ID)
	app.chatMu.Lock()
	got := app.followSessionID
	app.chatMu.Unlock()
	if got != sess.ID {
		t.Fatalf("followSessionID = %q, want %q", got, sess.ID)
	}

	rs, _ := mgr.Runs()
	now := time.Now()
	run := session.Run{ID: "remote", SessionID: sess.ID, State: session.RunStateRunning, OwnerInstance: "other", AgentID: "agent-x", StartedAt: now, UpdatedAt: now}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}
	_ = rs.AppendRunEvents([]session.RunEvent{
		{RunID: run.ID, SessionID: sess.ID, Type: session.RunEventStarted, AgentID: run.AgentID, Data: []byte(`{"base_seq":0}`)},
		{RunID: run.ID, SessionID: sess.ID, Type: session.AgentEventToken, AgentID: run.AgentID, Data: []byte(`{"content":"from-peer"}`)},
	})
	deadline := time.Now().Add(3 * time.Second)
	for {
		text, _ := transcriptText(app)
		if strings.Contains(text, "from-peer") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("new-session follower did not receive peer run: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSessionUpdatedSyncsModeAndClosesAsk(t *testing.T) {
	app, mgr := newSessionApp(t)
	orch := agent.New("orchestrator")
	app.agent = orch
	app.deps.Agent = orch
	sess, err := mgr.Create(session.CreateOptions{Mode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := mgr.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	msgs, _ := mgr.Messages(sess.ID)
	app.activateSession(settings.SessionInfo{ID: info.ID, Title: info.Title, Mode: info.Mode, Messages: msgs})

	app.onAgentAsk(agent.AgentAsk{AgentID: "a1", Questions: []agent.AskQuestion{{ID: "q", Question: "Q", Type: "text"}}})
	if !app.home.AskOpen() {
		t.Fatal("ask did not open")
	}
	app.onAgentAskAnswer(agent.AgentAskAnswer{AgentID: "a1", Answers: map[string]string{"q": "A"}})
	if app.home.AskOpen() {
		t.Fatal("ask was not closed by another client's answer")
	}

	if err := mgr.SetMode(sess.ID, "plan"); err != nil {
		t.Fatal(err)
	}
	app.onSessionUpdated(session.SessionUpdated{SessionID: sess.ID})
	if app.mode != agent.ModePlan {
		t.Fatalf("mode = %q, want plan", app.mode)
	}
	if orch.CurrentMode() != agent.ModePlan {
		t.Fatalf("agent mode = %q, want plan", orch.CurrentMode())
	}
}
