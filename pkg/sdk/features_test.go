package sdk

import (
	"context"
	"testing"
)

func openTestEngine(t *testing.T, provider string) *Engine {
	t.Helper()
	registerMock(provider)
	eng, err := Open(context.Background(), testOptions(t, provider))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func TestSessions(t *testing.T) {
	eng := openTestEngine(t, "sdk-sessions")

	s, err := eng.CreateSession(CreateSessionOptions{Title: "first", Provider: "sdk-sessions", Model: "mock-model"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if s.ID == "" {
		t.Fatal("empty session id")
	}

	got, err := eng.GetSession(s.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Title != "first" {
		t.Fatalf("Title = %q, want first", got.Title)
	}

	if err := eng.SetSessionTitle(s.ID, "renamed"); err != nil {
		t.Fatalf("SetSessionTitle: %v", err)
	}
	got, _ = eng.GetSession(s.ID)
	if got.Title != "renamed" {
		t.Fatalf("Title = %q, want renamed", got.Title)
	}

	msg, err := eng.AppendMessage(s.ID, UserMessage("hello"))
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if msg.Seq != 1 {
		t.Fatalf("Seq = %d, want 1", msg.Seq)
	}
	msgs, err := eng.SessionMessages(s.ID)
	if err != nil {
		t.Fatalf("SessionMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}

	fork, err := eng.ForkSession(s.ID, msg.ID)
	if err != nil {
		t.Fatalf("ForkSession: %v", err)
	}
	if fork.ParentID != s.ID {
		t.Fatalf("ParentID = %q, want %q", fork.ParentID, s.ID)
	}

	list, total, err := eng.ListSessions(1, 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if total < 2 {
		t.Fatalf("total = %d, want >= 2", total)
	}
	if len(list) == 0 {
		t.Fatal("empty session list")
	}

	if err := eng.DeleteSession(s.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := eng.GetSession(s.ID); err != ErrNoSession {
		t.Fatalf("GetSession after delete err = %v, want ErrNoSession", err)
	}
}

func TestComplete(t *testing.T) {
	eng := openTestEngine(t, "sdk-complete")

	resp, err := eng.Complete(context.Background(), CompleteRequest{
		Provider: "sdk-complete",
		Model:    "mock-model",
		Messages: []Message{UserMessage("hi")},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.GetContent() == "" {
		t.Fatal("empty content")
	}
}

func TestCompleteStream(t *testing.T) {
	eng := openTestEngine(t, "sdk-complete-stream")

	var chunks int
	err := eng.CompleteStream(context.Background(), CompleteRequest{
		Provider: "sdk-complete-stream",
		Model:    "mock-model",
		Messages: []Message{UserMessage("hi")},
	}, func(chunk StreamChunk) error {
		if chunk.Content != "" {
			chunks++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if chunks == 0 {
		t.Fatal("no chunks received")
	}
}

func TestModelsAndProviders(t *testing.T) {
	eng := openTestEngine(t, "sdk-models")

	models, err := eng.Models(context.Background(), "sdk-models")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}

	found := false
	for _, p := range eng.Providers() {
		if p == "sdk-models" {
			found = true
		}
	}
	if !found {
		t.Fatal("provider not listed")
	}
}

func TestFiles(t *testing.T) {
	eng := openTestEngine(t, "sdk-files")

	if _, err := eng.WriteFile("hello.txt", []byte("world\n")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, err := eng.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content == "" {
		t.Fatal("empty file content")
	}
	if _, err := eng.EditFile("hello.txt", "world", "earth", false); err != nil {
		t.Fatalf("EditFile: %v", err)
	}
	if _, err := eng.StatFile("hello.txt"); err != nil {
		t.Fatalf("StatFile: %v", err)
	}
	if err := eng.DeleteFile("hello.txt"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
}

func TestRegisterToolAndMiddleware(t *testing.T) {
	eng := openTestEngine(t, "sdk-registry")

	tool := NewTool("sdk-echo", "echo tool", map[string]any{
		"type":       "object",
		"properties": map[string]any{"text": map[string]any{"type": "string"}},
	}, func(ctx context.Context, args string) (string, error) {
		return args, nil
	})
	if err := eng.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool: %v", err)
	}
	names := map[string]bool{}
	for _, t := range eng.Tools() {
		names[t.Name()] = true
	}
	if !names["sdk-echo"] {
		t.Fatal("registered tool not listed")
	}
	if !eng.UnregisterTool("sdk-echo") {
		t.Fatal("UnregisterTool returned false")
	}

	mw := &baseMiddleware{}
	if err := eng.RegisterMiddleware("sdk-test-mw", mw); err != nil {
		t.Fatalf("RegisterMiddleware: %v", err)
	}
	if !eng.UnregisterMiddleware("sdk-test-mw") {
		t.Fatal("UnregisterMiddleware returned false")
	}
}

func TestSubscribeEvents(t *testing.T) {
	eng := openTestEngine(t, "sdk-events")

	got := make(chan AgentFinished, 1)
	unsub, err := eng.OnAgentFinished(func(ev AgentFinished) {
		select {
		case got <- ev:
		default:
		}
	})
	if err != nil {
		t.Fatalf("OnAgentFinished: %v", err)
	}
	defer unsub()

	_, err = eng.Chat(context.Background(), ChatRequest{
		Input:    "hello",
		Provider: "sdk-events",
		Model:    "mock-model",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	select {
	case ev := <-got:
		if ev.AgentName == "" {
			t.Fatal("empty agent name in event")
		}
	default:
		t.Fatal("expected agent.finished event")
	}
}

type baseMiddleware struct {
	BaseMiddleware
}

func TestSessionMode(t *testing.T) {
	eng := openTestEngine(t, "sdk-mode")

	s, err := eng.CreateSession(CreateSessionOptions{Title: "modes", Provider: "sdk-mode", Model: "mock-model", Mode: ModePlan})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if s.Mode != string(ModePlan) {
		t.Fatalf("Mode = %q, want plan", s.Mode)
	}

	if err := eng.SetSessionMode(s.ID, ModeBuild); err != nil {
		t.Fatalf("SetSessionMode: %v", err)
	}
	got, err := eng.GetSession(s.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Mode != string(ModeBuild) {
		t.Fatalf("Mode after SetSessionMode = %q, want build", got.Mode)
	}
}
