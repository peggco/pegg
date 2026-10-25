package tui

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

type gateProvider struct {
	name    string
	reply   string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *gateProvider) Name() string { return p.name }

func (p *gateProvider) Chat(context.Context, *llm.Request) (*llm.Response, error) {
	return &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{Role: llm.RoleAssistant, Content: p.reply}}}}, nil
}

func (p *gateProvider) ChatStream(ctx context.Context, _ *llm.Request, handler llm.StreamHandler) error {
	p.once.Do(func() { close(p.started) })
	select {
	case <-p.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := handler(llm.StreamChunk{Content: p.reply}); err != nil {
		return err
	}
	return handler(llm.StreamChunk{IsDone: true, FinishReason: llm.FinishReasonStop})
}

func (p *gateProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }

func TestConcurrentSessionsDoNotLeakEvents(t *testing.T) {
	app, mgr := newSessionApp(t)
	app.ctx = context.Background()
	app.bus = app.deps.Bus

	provA := &gateProvider{name: "prov-a", reply: "answer-a", started: make(chan struct{}), release: make(chan struct{})}
	provB := &gateProvider{name: "prov-b", reply: "answer-b", started: make(chan struct{}), release: make(chan struct{})}
	agentA := agent.New("orch", agent.WithProvider(provA), agent.WithModel(llm.Model{ID: "model-a"}))
	agentB := agent.New("orch", agent.WithProvider(provB), agent.WithModel(llm.Model{ID: "model-b"}))

	app.deps.Agent = agentA
	app.deps.AgentFactory = func() (*agent.Agent, error) { return agentB, nil }
	app.agent = agentA
	app.viewID = agentA.ID
	if err := app.subscribeChat(app.bus); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.unsubscribeChat(app.bus) })

	go app.runAgentWithAttachments("alpha", nil)
	select {
	case <-provA.started:
	case <-time.After(3 * time.Second):
		t.Fatal("session A did not start")
	}

	app.chatMu.Lock()
	sessionA := app.session.info.ID
	rtA := app.runtimes[sessionA]
	app.chatMu.Unlock()
	if rtA == nil || !rtA.running {
		t.Fatal("session A runtime should be running")
	}

	infoB, err := mgr.Create(session.CreateOptions{Title: "B", Provider: "prov-b", Model: "model-b"})
	if err != nil {
		t.Fatal(err)
	}
	msgsB, _ := mgr.Messages(infoB.ID)
	app.activateSession(settings.SessionInfo{ID: infoB.ID, Title: infoB.Title, Provider: infoB.Provider, Model: infoB.Model, Messages: msgsB})

	go app.runAgentWithAttachments("beta", nil)
	select {
	case <-provB.started:
	case <-time.After(3 * time.Second):
		t.Fatal("session B did not start")
	}
	close(provB.release)

	waitFor(t, 3*time.Second, func() bool {
		app.chatMu.Lock()
		defer app.chatMu.Unlock()
		rtB := app.runtimes[infoB.ID]
		return rtB != nil && !rtB.running
	})

	close(provA.release)
	waitFor(t, 3*time.Second, func() bool {
		app.chatMu.Lock()
		defer app.chatMu.Unlock()
		return !rtA.running
	})

	app.chatMu.Lock()
	defer app.chatMu.Unlock()
	var texts []string
	if app.main != nil {
		for _, it := range app.main.items {
			texts = append(texts, it.Text)
		}
	}
	joined := strings.Join(texts, "\n")
	if strings.Contains(joined, "alpha") || strings.Contains(joined, "answer-a") {
		t.Fatalf("session A events leaked into session B view: %q", joined)
	}
	if !strings.Contains(joined, "beta") {
		t.Fatalf("session B view missing its own input: %q", joined)
	}
	var historyA []string
	for _, m := range rtA.history {
		if text, ok := m.Content.(string); ok {
			historyA = append(historyA, text)
		}
	}
	if !strings.Contains(strings.Join(historyA, "\n"), "answer-a") {
		t.Fatalf("session A history was not preserved: %v", historyA)
	}
}
