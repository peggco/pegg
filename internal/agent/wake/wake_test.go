package wake

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/reminder"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

type fakeProvider struct {
	mu       sync.Mutex
	requests []*llm.Request
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Chat(_ context.Context, req *llm.Request) (*llm.Response, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	return &llm.Response{Choices: []llm.Choice{{
		Message: &llm.Message{Role: llm.RoleAssistant, Content: "done"},
	}}}, nil
}
func (p *fakeProvider) ChatStream(_ context.Context, req *llm.Request, handler llm.StreamHandler) error {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if err := handler(llm.StreamChunk{Content: "done"}); err != nil {
		return err
	}
	return handler(llm.StreamChunk{IsDone: true, FinishReason: llm.FinishReasonStop})
}
func (p *fakeProvider) ListModels(context.Context) ([]llm.Model, error) { return nil, nil }

func (p *fakeProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func (p *fakeProvider) last() *llm.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.requests) == 0 {
		return nil
	}
	return p.requests[len(p.requests)-1]
}

type harness struct {
	waker    *Waker
	manager  *session.Manager
	runs     *runpkg.Coordinator
	provider *fakeProvider
	agent    *agent.Agent
	session  *session.Session
	bus      event.Bus
}

func newHarness(t *testing.T) *harness {
	t.Helper()
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

	prov := &fakeProvider{}
	orch := agent.New("orchestrator",
		agent.WithProvider(prov),
		agent.WithModel(llm.Model{ID: "m"}),
		agent.WithSystemPrompt("sys"),
	)
	orch.Bus = bus

	sess, err := mgr.Create(session.CreateOptions{Provider: "fake", Model: "m", ProjectDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.UserMessage("earlier turn")); err != nil {
		t.Fatal(err)
	}

	w := New(Deps{Bus: bus, Runs: coord, Sessions: mgr, Log: log})
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)

	return &harness{waker: w, manager: mgr, runs: coord, provider: prov, agent: orch, session: sess, bus: bus}
}

func (h *harness) notify(t *testing.T, name, output string) {
	t.Helper()
	h.agent.QueueNotification(reminder.New("subagent", output, "agent", name))
	h.bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
		ParentAgentID: h.agent.ID,
		Parent:        h.agent,
		SessionID:     h.session.ID,
		SubAgentName:  name,
		Output:        output,
	})
}

func requestText(req *llm.Request) string {
	if req == nil {
		return ""
	}
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(fmt.Sprint(m.Content))
		b.WriteByte('\n')
	}
	return b.String()
}

func waitForRequest(t *testing.T, p *fakeProvider) *llm.Request {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if req := p.last(); req != nil {
			return req
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("waker did not continue the agent")
	return nil
}

func TestWakerContinuesIdleParent(t *testing.T) {
	h := newHarness(t)
	h.notify(t, "bg-1", "background work done")

	req := waitForRequest(t, h.provider)
	text := requestText(req)
	if !strings.Contains(text, "background work done") {
		t.Fatalf("request missing the system reminder:\n%s", text)
	}
	if !strings.Contains(text, "earlier turn") {
		t.Fatalf("request missing the persisted history:\n%s", text)
	}
}

func TestWakerSkipsWithoutPendingNotifications(t *testing.T) {
	h := newHarness(t)
	h.bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
		ParentAgentID: h.agent.ID,
		Parent:        h.agent,
		SessionID:     h.session.ID,
		SubAgentName:  "bg-1",
	})
	time.Sleep(400 * time.Millisecond)
	if n := h.provider.count(); n != 0 {
		t.Fatalf("waker continued without a pending notification (%d requests)", n)
	}
}

func TestWakerWaitsForActiveRun(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	lease, err := h.runs.Claim(ctx, runpkg.ClaimRequest{
		SessionID: h.session.ID,
		AgentID:   h.agent.ID,
		AgentName: h.agent.Name,
		Provider:  "fake",
		Model:     "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	h.notify(t, "bg-1", "late wake")

	time.Sleep(400 * time.Millisecond)
	if n := h.provider.count(); n != 0 {
		t.Fatalf("waker started a run while another was active (%d requests)", n)
	}

	lease.Finish(session.RunStateDone, "")

	req := waitForRequest(t, h.provider)
	if !strings.Contains(requestText(req), "late wake") {
		t.Fatalf("wake request missing reminder:\n%s", requestText(req))
	}
}

func TestWakerCoalescesNotifications(t *testing.T) {
	h := newHarness(t)
	h.agent.QueueNotification(reminder.New("subagent", "first done", "agent", "bg-1"))
	h.agent.QueueNotification(reminder.New("subagent", "second done", "agent", "bg-2"))
	h.bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
		ParentAgentID: h.agent.ID,
		Parent:        h.agent,
		SessionID:     h.session.ID,
		SubAgentName:  "bg-1",
	})
	h.bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
		ParentAgentID: h.agent.ID,
		Parent:        h.agent,
		SessionID:     h.session.ID,
		SubAgentName:  "bg-2",
	})

	req := waitForRequest(t, h.provider)
	text := requestText(req)
	if !strings.Contains(text, "first done") || !strings.Contains(text, "second done") {
		t.Fatalf("request did not drain both reminders:\n%s", text)
	}
	if n := h.provider.count(); n != 1 {
		t.Fatalf("expected one coalesced run, got %d", n)
	}
}

func TestWakerIgnoresMissingSession(t *testing.T) {
	h := newHarness(t)
	h.agent.QueueNotification(reminder.New("subagent", "orphan", "agent", "bg-1"))
	h.bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
		ParentAgentID: h.agent.ID,
		Parent:        h.agent,
		SessionID:     "missing-session",
		SubAgentName:  "bg-1",
	})
	time.Sleep(400 * time.Millisecond)
	if n := h.provider.count(); n != 0 {
		t.Fatalf("waker continued a missing session (%d requests)", n)
	}
}

func TestWakerUsesPersistedTaskResult(t *testing.T) {
	h := newHarness(t)
	if _, err := h.manager.AppendMessage(h.session.ID, llm.ToolMessage("final subagent answer", "call-task")); err != nil {
		t.Fatal(err)
	}
	h.notify(t, "bg-1", "background work done")

	req := waitForRequest(t, h.provider)
	if text := requestText(req); !strings.Contains(text, "final subagent answer") {
		t.Fatalf("request missing the persisted task result:\n%s", text)
	}
}
