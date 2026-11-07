package acp

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
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
				if !strings.Contains(raw, `"method":"_pegg/ask"`) {
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

	linkPath := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(linkPath, []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, atts, err := srv.preparePrompt(context.Background(), sess, []ContentBlock{
		{Type: "text", Text: "look"},
		{Type: "image", MimeType: "image/png", Name: "shot.png", Data: base64.StdEncoding.EncodeToString([]byte("png-bytes"))},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///tmp/a.txt", MimeType: "text/plain", Text: "contents"}},
		{Type: "resource", Resource: &EmbeddedResource{
			URI:      "file:///tmp/report.pdf",
			MimeType: "application/pdf",
			Blob:     base64.StdEncoding.EncodeToString([]byte("%PDF-1.7")),
		}},
		{Type: "resource_link", URI: "file://" + linkPath},
	})
	if err != nil {
		t.Fatalf("preparePrompt: %v", err)
	}
	if input != "look" {
		t.Fatalf("input = %q, want just the prompt text", input)
	}
	if len(atts) != 4 {
		t.Fatalf("attachments = %+v, want 4", atts)
	}

	wantTypes := []llm.AttachmentType{
		llm.AttachmentTypeImage,
		llm.AttachmentTypeFile,
		llm.AttachmentTypeFile,
		llm.AttachmentTypeFile,
	}
	wantNames := []string{"shot.png", "a.txt", "report.pdf", "notes.md"}
	for i, att := range atts {
		if att.Type != wantTypes[i] {
			t.Fatalf("attachment %d type = %q, want %q", i, att.Type, wantTypes[i])
		}
		if att.FileName != wantNames[i] {
			t.Fatalf("attachment %d name = %q, want %q", i, att.FileName, wantNames[i])
		}
		if att.Path == "" || !attach.IsStorePath(att.Path) {
			t.Fatalf("attachment %d not stored: %+v", i, att)
		}
		if att.Data == "" {
			t.Fatalf("attachment %d has no payload: %+v", i, att)
		}
	}
	if atts[1].MediaType != "text/plain" {
		t.Fatalf("text media type = %q", atts[1].MediaType)
	}
	if atts[2].MediaType != "application/pdf" {
		t.Fatalf("pdf media type = %q", atts[2].MediaType)
	}
	stored, err := os.ReadFile(atts[1].Path)
	if err != nil || string(stored) != "contents" {
		t.Fatalf("stored text = %q, err %v", stored, err)
	}
	if got := attach.Block(atts); !strings.Contains(got, atts[0].Path) || !strings.Contains(got, "shot.png") {
		t.Fatalf("block = %q, want stored paths and names", got)
	}
}

func TestPreparePromptImageWithoutName(t *testing.T) {
	srv := newTestServer(t)
	sess := &ACPSession{ID: "s", Cwd: t.TempDir()}
	_, atts, err := srv.preparePrompt(context.Background(), sess, []ContentBlock{
		{Type: "image", MimeType: "image/jpeg", Data: base64.StdEncoding.EncodeToString([]byte("jpeg"))},
	})
	if err != nil {
		t.Fatalf("preparePrompt: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("attachments = %+v", atts)
	}
	if atts[0].FileName != "file.jpg" || atts[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("attachment = %+v, want synthesized image name", atts[0])
	}
}

func TestHistoryEmitsAttachmentChips(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt

	msg := session.Message{
		ID:   "m1",
		Seq:  1,
		Role: llm.RoleUser,
		Content: llm.Content{
			Text: "<attachments>\n<attachment name=\"r.pdf\" path=\"/tmp/pegg/r.pdf\"/>\n</attachments>",
			Attachments: []llm.Attachment{{
				Type:      llm.AttachmentTypeFile,
				MediaType: "application/pdf",
				FileName:  "r.pdf",
				Path:      "/tmp/pegg/r.pdf",
			}},
		},
	}
	srv.emitHistoryMessages("s1", "/tmp", []session.Message{msg}, true)

	raw := bt.last()
	if raw == "" {
		t.Fatal("no history update emitted")
	}
	var notification struct {
		Params struct {
			Update SessionUpdate `json:"update"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(raw), &notification); err != nil {
		t.Fatalf("unmarshal update: %v", err)
	}
	update := notification.Params.Update
	if update.SessionUpdate != "user_message_chunk" {
		t.Fatalf("sessionUpdate = %q", update.SessionUpdate)
	}
	atts, _ := update.Meta["com.pegg/attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("attachments = %+v", update.Meta)
	}
	att, _ := atts[0].(map[string]any)
	if att["name"] != "r.pdf" || att["type"] != "file" {
		t.Fatalf("attachment = %+v", att)
	}
}

func TestAttachmentInfos(t *testing.T) {
	atts := []llm.Attachment{
		{Type: llm.AttachmentTypeImage, MediaType: "image/png", FileName: "shot.png", Path: "/tmp/pegg/shot.png", Data: "aGVsbG8="},
		{Type: llm.AttachmentTypeFile, MediaType: "application/pdf", Path: "/tmp/pegg/report.pdf"},
	}
	infos := attachmentInfos(atts)
	if len(infos) != 2 {
		t.Fatalf("infos = %+v", infos)
	}
	if infos[0].Name != "shot.png" || infos[0].Type != "image" || infos[0].Size != 5 {
		t.Fatalf("first info = %+v", infos[0])
	}
	if infos[1].Name != "report.pdf" || infos[1].Type != "file" {
		t.Fatalf("second info = %+v", infos[1])
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
	if len(locs) != 1 || locs[0].Path != "/tmp/a.go" || locs[0].Line == nil || *locs[0].Line != 4 {
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
	if peggSessionID(created) == "" {
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
	if items[0].Cwd != "/tmp/project" {
		t.Fatalf("cwd = %q", items[0].Cwd)
	}
	if items[0].Meta["com.pegg/mode"] != string(agent.ModeAuto) {
		t.Fatalf("mode = %v", items[0].Meta["com.pegg/mode"])
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
