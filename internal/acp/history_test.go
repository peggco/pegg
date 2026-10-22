package acp

import (
	"context"
	"strings"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func seedHistorySession(t *testing.T, srv *Server) (string, []session.Message) {
	t.Helper()
	sess, err := srv.sessions.Create(session.CreateOptions{Title: "S", ProjectDir: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []llm.Message{
		llm.UserMessage("first"),
		llm.AssistantMessage("answer one"),
		llm.UserMessage("second"),
		llm.AssistantMessage("answer two"),
	}
	msgs := make([]session.Message, 0, len(inputs))
	for _, input := range inputs {
		recorded, err := srv.sessions.AppendMessage(sess.ID, input)
		if err != nil {
			t.Fatal(err)
		}
		msgs = append(msgs, *recorded)
	}
	srv.addSession(&ACPSession{
		ID:            "acp-1",
		Cwd:           "/tmp",
		PeggSessionID: sess.ID,
	})
	return sess.ID, msgs
}

func TestSessionForkBeforeMessage(t *testing.T) {
	srv := newTestServer(t)
	parentID, msgs := seedHistorySession(t, srv)

	index := 1
	params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", UserMessageIndex: &index})
	raw, err := srv.handleSessionFork(context.Background(), json.RawMessage(params))
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(SessionForkResult)
	if !ok {
		t.Fatalf("result = %T", raw)
	}
	if result.SessionId == "" || result.SessionId == parentID {
		t.Fatalf("fork session id = %q", result.SessionId)
	}
	if !strings.HasSuffix(result.Title, "- fork") {
		t.Fatalf("fork title = %q", result.Title)
	}
	if result.ParentId != parentID {
		t.Fatalf("parent = %q, want %q", result.ParentId, parentID)
	}

	forked, err := srv.sessions.Messages(result.SessionId)
	if err != nil {
		t.Fatal(err)
	}
	if len(forked) != 2 {
		t.Fatalf("fork messages = %d, want 2", len(forked))
	}
	if forked[0].Content != "first" || forked[1].Content != "answer one" {
		t.Fatalf("fork messages = %+v", forked)
	}
	if forked[0].ID == msgs[0].ID {
		t.Fatal("forked messages must be copies, not the original rows")
	}
}

func TestSessionForkByMessageID(t *testing.T) {
	srv := newTestServer(t)
	_, msgs := seedHistorySession(t, srv)

	params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", MessageId: msgs[2].ID})
	raw, err := srv.handleSessionFork(context.Background(), json.RawMessage(params))
	if err != nil {
		t.Fatal(err)
	}
	result := raw.(SessionForkResult)
	forked, err := srv.sessions.Messages(result.SessionId)
	if err != nil {
		t.Fatal(err)
	}
	if len(forked) != 2 {
		t.Fatalf("fork messages = %d, want 2", len(forked))
	}
}

func TestSessionRevertRemovesTargetAndAfter(t *testing.T) {
	srv := newTestServer(t)
	parentID, _ := seedHistorySession(t, srv)

	index := 1
	params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", UserMessageIndex: &index})
	raw, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params))
	if err != nil {
		t.Fatal(err)
	}
	result := raw.(SessionRevertResult)
	if result.Removed != 2 {
		t.Fatalf("removed = %d, want 2", result.Removed)
	}

	remaining, err := srv.sessions.Messages(parentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining = %+v", remaining)
	}
	if remaining[0].Content != "first" || remaining[1].Content != "answer one" {
		t.Fatalf("remaining = %+v", remaining)
	}
}

func TestSessionRevertFirstMessageEmptiesHistory(t *testing.T) {
	srv := newTestServer(t)
	parentID, _ := seedHistorySession(t, srv)

	index := 0
	params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", UserMessageIndex: &index})
	if _, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params)); err != nil {
		t.Fatal(err)
	}
	remaining, err := srv.sessions.Messages(parentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining = %+v", remaining)
	}
}

func TestSessionMessageRefErrors(t *testing.T) {
	srv := newTestServer(t)
	seedHistorySession(t, srv)

	t.Run("unknown session", func(t *testing.T) {
		index := 0
		params, _ := json.Marshal(SessionMessageRef{SessionId: "nope", UserMessageIndex: &index})
		if _, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		index := 9
		params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", UserMessageIndex: &index})
		if _, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("unknown message id falls back to nothing", func(t *testing.T) {
		params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", MessageId: "nope"})
		if _, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejected while a prompt is running", func(t *testing.T) {
		sess, ok := srv.getSession("acp-1")
		if !ok {
			t.Fatal("session missing")
		}
		_, cancel := context.WithCancel(context.Background())
		defer cancel()
		if !sess.StartPrompt(context.Background(), cancel, "req") {
			t.Fatal("could not start the prompt")
		}
		defer sess.ClearPrompt()

		index := 0
		params, _ := json.Marshal(SessionMessageRef{SessionId: "acp-1", UserMessageIndex: &index})
		if _, err := srv.handleSessionRevert(context.Background(), json.RawMessage(params)); err == nil {
			t.Fatal("expected an error while a prompt is running")
		}
	})
}
