package acp

import (
	"context"
	"strings"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
)

func testSession(t *testing.T, srv *Server, id string) *ACPSession {
	t.Helper()
	a := agent.New("test-agent")
	a.Bus = srv.bus
	sess := &ACPSession{ID: id, Agent: a, Cwd: t.TempDir()}
	srv.addSession(sess)
	return sess
}

func TestAgentAskPermissionAllows(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt
	sess := testSession(t, srv, "s1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !sess.StartPrompt(ctx, cancel, 1) {
		t.Fatal("start prompt")
	}
	defer sess.ClearPrompt()

	answers := make(chan agent.AgentAskAnswer, 1)
	_ = srv.bus.Subscribe(agent.TopicAgentAskAnswer, func(e agent.AgentAskAnswer) {
		answers <- e
	})

	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if bt.count() > 0 {
				raw := bt.last()
				var req struct {
					ID string `json:"id"`
				}
				if json.Unmarshal([]byte(raw), &req) == nil && req.ID != "" {
					resp, _ := json.Marshal(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"result": map[string]any{
							"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"},
						},
					})
					srv.handleFrame(ctx, Message{Body: resp})
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()

	srv.handleAgentAsk(agent.AgentAsk{
		AgentID:   sess.Agent.ID,
		AgentName: "orchestrator",
		Questions: []agent.AskQuestion{{
			ID:       permissionQuestionID,
			Question: "Allow bash?",
			Type:     "select",
			Options:  []string{"Allow", "Allow All", "Reject"},
		}},
	})

	select {
	case got := <-answers:
		if got.Answers["decision"] != "Allow" {
			t.Fatalf("decision = %q, want Allow", got.Answers["decision"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no answer published")
	}
	if !strings.Contains(bt.last(), "session/request_permission") {
		t.Fatalf("expected permission request, got %s", bt.last())
	}
}

func TestAgentAskCustomQuestions(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt
	sess := testSession(t, srv, "s2")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !sess.StartPrompt(ctx, cancel, 1) {
		t.Fatal("start prompt")
	}
	defer sess.ClearPrompt()

	answers := make(chan agent.AgentAskAnswer, 1)
	_ = srv.bus.Subscribe(agent.TopicAgentAskAnswer, func(e agent.AgentAskAnswer) {
		answers <- e
	})

	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if bt.count() > 0 {
				raw := bt.last()
				if !strings.Contains(raw, `"method":"pegg/ask"`) {
					time.Sleep(time.Millisecond)
					continue
				}
				var req struct {
					ID string `json:"id"`
				}
				if json.Unmarshal([]byte(raw), &req) == nil && req.ID != "" {
					resp, _ := json.Marshal(map[string]any{
						"jsonrpc": "2.0",
						"id":      req.ID,
						"result":  map[string]any{"answers": map[string]string{"q1": "yes"}},
					})
					srv.handleFrame(ctx, Message{Body: resp})
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()

	srv.handleAgentAsk(agent.AgentAsk{
		AgentID: sess.Agent.ID,
		Questions: []agent.AskQuestion{{
			ID:       "q1",
			Question: "Continue?",
			Type:     "text",
		}},
	})

	select {
	case got := <-answers:
		if got.Answers["q1"] != "yes" {
			t.Fatalf("answer = %q, want yes", got.Answers["q1"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no answer published")
	}
}

func TestPreparePromptAttachments(t *testing.T) {
	srv := newTestServer(t)
	sess := &ACPSession{ID: "s", Cwd: t.TempDir()}
	input, atts, err := srv.preparePrompt(context.Background(), sess, []ContentBlock{
		{Type: "text", Text: "look"},
		{Type: "image", MimeType: "image/png", Data: "aGVsbG8="},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///tmp/a.txt", Text: "contents"}},
	})
	if err != nil {
		t.Fatalf("preparePrompt: %v", err)
	}
	if !strings.Contains(input, "look") || !strings.Contains(input, "contents") {
		t.Fatalf("unexpected input: %q", input)
	}
	if len(atts) != 1 || atts[0].Type != "image" {
		t.Fatalf("unexpected attachments: %+v", atts)
	}
}

func TestToolMetadata(t *testing.T) {
	if toolKind("bash") != ToolKindExecute {
		t.Fatal("bash kind")
	}
	if toolKind("edit") != ToolKindEdit {
		t.Fatal("edit kind")
	}
	args := parseArgs(`{"filePath":"/tmp/a.go","line":4}`)
	locs := toolLocations("/tmp", args)
	if len(locs) != 1 || locs[0].Path != "/tmp/a.go" || locs[0].Line != 4 {
		t.Fatalf("unexpected locations: %+v", locs)
	}
	entries, ok := planEntries(parseArgs(`{"todos":[{"title":"one","status":"pending","priority":"high"}]}`))
	if !ok || len(entries) != 1 || entries[0].Content != "one" {
		t.Fatalf("unexpected plan entries: %+v", entries)
	}
}

func TestACPRecorderCommandCustom(t *testing.T) {
	args, err := acpRecorderCommand(&config.STTConfig{RecordCommand: "echo {output}"}, "/tmp/voice.wav")
	if err != nil {
		t.Fatalf("acpRecorderCommand: %v", err)
	}
	if len(args) < 2 || args[0] != "echo" || args[len(args)-1] != "/tmp/voice.wav" {
		t.Fatalf("unexpected recorder command: %+v", args)
	}
}

func TestSessionListMetadata(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	res, err := srv.handleSessionNew(ctx, toJSON(t, SessionNewParams{Cwd: "/tmp/project"}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	created := res.(SessionNewResult)
	if created.PeggSessionId == "" {
		t.Fatal("expected pegg session id")
	}
	lr, err := srv.handleSessionList(ctx, toJSON(t, SessionListParams{Cwd: "/tmp/project"}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	items := lr.(SessionListResult).Sessions
	if len(items) != 1 {
		t.Fatalf("sessions = %d, want 1", len(items))
	}
	if items[0].ProjectDir != "/tmp/project" {
		t.Fatalf("project dir = %q", items[0].ProjectDir)
	}
	if items[0].Mode != string(agent.ModeAuto) {
		t.Fatalf("mode = %q", items[0].Mode)
	}
}

func TestSubagentAskRoutesToParentSession(t *testing.T) {
	srv := newTestServer(t)
	sess := testSession(t, srv, "s3")
	orch := sess.Agent
	srv.handleAgentStarted(agent.AgentStarted{AgentID: "sub-1", ParentAgentID: orch.ID})
	got, ok := srv.sessionForAgent("sub-1")
	if !ok || got.ID != sess.ID {
		t.Fatalf("subagent did not map to session: %+v %v", got, ok)
	}
	srv.removeSession(sess.ID)
	if _, ok := srv.sessionForAgent("sub-1"); ok {
		t.Fatal("subagent mapping was not cleaned up")
	}
}
