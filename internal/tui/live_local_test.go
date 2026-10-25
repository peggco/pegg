package tui

import (
	"context"
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

type staticProvider struct {
	name  string
	reply string
}

func (p *staticProvider) Name() string { return p.name }

func (p *staticProvider) Chat(context.Context, *llm.Request) (*llm.Response, error) {
	return &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{Role: llm.RoleAssistant, Content: p.reply}}}}, nil
}

func (p *staticProvider) ChatStream(_ context.Context, _ *llm.Request, handler llm.StreamHandler) error {
	if err := handler(llm.StreamChunk{Content: p.reply}); err != nil {
		return err
	}
	return handler(llm.StreamChunk{IsDone: true, FinishReason: llm.FinishReasonStop})
}

func (p *staticProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }

func TestLocalRunStreamsOnceWithLiveHub(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
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
	rec := session.NewRecorder(mgr, log)
	if err := rec.Start(bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rec.Stop(bus) })

	sess, err := mgr.Create(session.CreateOptions{Provider: "prov-live", Model: "model-live"})
	if err != nil {
		t.Fatal(err)
	}
	orch := agent.New("orch",
		agent.WithProvider(&staticProvider{name: "prov-live", reply: "hello-once"}),
		agent.WithModel(llm.Model{ID: "model-live"}),
	)
	app := &App{
		bus: bus, deps: settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent: orch, chat: components.NewChat(), ctx: context.Background(), mode: agent.ModeAuto,
		subs: map[string]*agentTranscript{}, subItemByID: map[string]*components.ChatItem{},
	}
	t.Cleanup(app.stopAttach)
	if err := app.subscribeChat(bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.unsubscribeChat(bus) })

	msgs, _ := mgr.Messages(sess.ID)
	app.activateSession(settings.SessionInfo{ID: sess.ID, Title: sess.Title, Provider: sess.Provider, Model: sess.Model, Messages: msgs})

	go app.runAgentWithAttachments("hi-local", nil)
	waitFor(t, 5*time.Second, func() bool {
		text, _ := transcriptText(app)
		app.chatMu.Lock()
		running := app.running
		attached := app.attachRunID != ""
		app.chatMu.Unlock()
		return strings.Contains(text, "hello-once") && !running && !attached
	})

	text, items := transcriptText(app)
	if got := strings.Count(text, "hello-once"); got != 1 {
		t.Fatalf("assistant text streamed %d times: %q", got, text)
	}
	users := 0
	for _, it := range items {
		if it.Kind == components.ItemUser && it.Text == "hi-local" {
			users++
		}
	}
	if users != 1 {
		t.Fatalf("user input rendered %d times", users)
	}

	go app.runAgentWithAttachments("again-local", nil)
	waitFor(t, 5*time.Second, func() bool {
		text, _ := transcriptText(app)
		app.chatMu.Lock()
		running := app.running
		attached := app.attachRunID != ""
		app.chatMu.Unlock()
		return strings.Count(text, "hello-once") == 2 && !running && !attached
	})
	text, items = transcriptText(app)
	if got := strings.Count(text, "hello-once"); got != 2 {
		t.Fatalf("second run assistant text count = %d: %q", got, text)
	}
	for _, want := range []string{"hi-local", "again-local"} {
		users = 0
		for _, it := range items {
			if it.Kind == components.ItemUser && it.Text == want {
				users++
			}
		}
		if users != 1 {
			t.Fatalf("user input %q rendered %d times", want, users)
		}
	}
}

func TestSwitchBackToRunningLocalSessionReplaysOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStore()
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
	rec := session.NewRecorder(mgr, log)
	if err := rec.Start(bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rec.Stop(bus) })

	sessA, err := mgr.Create(session.CreateOptions{Provider: "prov-a", Model: "model-a"})
	if err != nil {
		t.Fatal(err)
	}
	prov := &gateProvider{name: "prov-a", reply: "answer-once", started: make(chan struct{}), release: make(chan struct{})}
	orch := agent.New("orch", agent.WithProvider(prov), agent.WithModel(llm.Model{ID: "model-a"}))
	app := &App{
		bus: bus, deps: settings.Deps{Sessions: mgr, Runs: coord, Live: hub, Agent: orch, Bus: bus},
		agent: orch, chat: components.NewChat(), ctx: context.Background(), mode: agent.ModeAuto,
		subs: map[string]*agentTranscript{}, subItemByID: map[string]*components.ChatItem{},
	}
	t.Cleanup(app.stopAttach)
	if err := app.subscribeChat(bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.unsubscribeChat(bus) })

	msgsA, _ := mgr.Messages(sessA.ID)
	app.activateSession(settings.SessionInfo{ID: sessA.ID, Title: sessA.Title, Provider: sessA.Provider, Model: sessA.Model, Messages: msgsA})
	go app.runAgentWithAttachments("run-a", nil)
	select {
	case <-prov.started:
	case <-time.After(3 * time.Second):
		t.Fatal("run did not start")
	}

	sessB, err := mgr.Create(session.CreateOptions{Provider: "prov-b", Model: "model-b"})
	if err != nil {
		t.Fatal(err)
	}
	msgsB, _ := mgr.Messages(sessB.ID)
	app.activateSession(settings.SessionInfo{ID: sessB.ID, Title: sessB.Title, Provider: sessB.Provider, Model: sessB.Model, Messages: msgsB})

	app.activateSession(settings.SessionInfo{ID: sessA.ID, Title: sessA.Title, Provider: sessA.Provider, Model: sessA.Model, Messages: msgsA})
	close(prov.release)
	waitFor(t, 5*time.Second, func() bool {
		text, _ := transcriptText(app)
		app.chatMu.Lock()
		running := app.running
		attached := app.attachRunID != ""
		app.chatMu.Unlock()
		return strings.Contains(text, "answer-once") && !running && !attached
	})
	time.Sleep(150 * time.Millisecond)
	text, items := transcriptText(app)
	if got := strings.Count(text, "answer-once"); got != 1 {
		t.Fatalf("replayed answer %d times: %q", got, text)
	}
	users := 0
	for _, it := range items {
		if it.Kind == components.ItemUser && it.Text == "run-a" {
			users++
		}
	}
	if users != 1 {
		t.Fatalf("replayed user input %d times", users)
	}
}
