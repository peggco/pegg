package acp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

type duplicateProvider struct{}

func (p *duplicateProvider) Name() string { return "duplicate-test" }

func (p *duplicateProvider) ListModels(context.Context) ([]llm.Model, error) {
	return []llm.Model{{ID: "mock-model"}}, nil
}

func (p *duplicateProvider) Chat(context.Context, *llm.Request) (*llm.Response, error) {
	return &llm.Response{Model: "mock-model"}, nil
}

func (p *duplicateProvider) ChatStream(_ context.Context, _ *llm.Request, handler llm.StreamHandler) error {
	for _, part := range []string{"hi, ", "how can I help"} {
		if err := handler(llm.StreamChunk{Content: part}); err != nil {
			return err
		}
	}
	return handler(llm.StreamChunk{FinishReason: llm.FinishReasonStop})
}

func TestFastLocalPromptDoesNotReplayIntoTheSameClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStoreAt(filepath.Join(t.TempDir(), "sessions.db"))
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
	hub := live.New(coord, mgr, bus, log)
	t.Cleanup(hub.Close)

	srv := New(config.DefaultConfig(), bus, log, nil, mgr, nil, coord, hub)
	tr := newBufferTransport()
	srv.transport = tr
	srv.newAgentFn = func() (*agent.Agent, error) {
		a := agent.New("orchestrator",
			agent.WithModel(llm.Model{ID: "mock-model"}),
			agent.WithProvider(&duplicateProvider{}),
		)
		a.Bus = bus
		return a, nil
	}

	result, err := srv.handleSessionNew(context.Background(), toJSON(t, SessionNewParams{Cwd: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	sessionID := result.(SessionNewResult).SessionId

	_, err = srv.handleSessionPrompt(context.Background(), toJSON(t, SessionPromptParams{
		SessionId: sessionID,
		Prompt:    []ContentBlock{{Type: "text", Text: "hi"}},
	}), "req-1")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(750 * time.Millisecond)

	tr.mu.Lock()
	raws := append([]string(nil), tr.sends...)
	tr.mu.Unlock()

	var assistant strings.Builder
	for _, raw := range raws {
		var msg struct {
			Method string `json:"method"`
			Params struct {
				Update struct {
					SessionUpdate string `json:"sessionUpdate"`
					Content       *struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"update"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(raw), &msg) != nil || msg.Method != "session/update" {
			continue
		}
		if msg.Params.Update.SessionUpdate == "agent_message_chunk" && msg.Params.Update.Content != nil {
			assistant.WriteString(msg.Params.Update.Content.Text)
		}
	}

	if got, want := assistant.String(), "hi, how can I help"; got != want {
		t.Fatalf("assistant text = %q, want %q (duplicate replay)", got, want)
	}
}
