package connect

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

func newFileTestConnector(t *testing.T) (*Connector, *vfs.VFS, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	fs, err := vfs.New(dir, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	c, err := New(Deps{Config: cfg, VFS: fs, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c, fs, dir
}

func TestFileDownloadChunksDeliveredFile(t *testing.T) {
	c, _, dir := newFileTestConnector(t)
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.rememberDelivered("s1", []llm.Attachment{{
		Type:      llm.AttachmentTypeFile,
		FileName:  "report.txt",
		MediaType: "text/plain",
		Path:      path,
		Size:      11,
	}})

	res, err := c.handleFileDownload(t.Context(), &Request{Params: []byte(
		`{"session_id":"s1","path":"report.txt","length":5}`,
	)})
	if err != nil {
		t.Fatal(err)
	}
	first := res.(fileChunk)
	if first.EOF || first.Length != 5 || first.Size != 11 {
		t.Fatalf("unexpected first chunk: %+v", first)
	}
	if got, _ := base64.StdEncoding.DecodeString(first.Data); string(got) != "hello" {
		t.Fatalf("first chunk data = %q", got)
	}

	res, err = c.handleFileDownload(t.Context(), &Request{Params: []byte(
		`{"session_id":"s1","path":"report.txt","offset":5,"length":64}`,
	)})
	if err != nil {
		t.Fatal(err)
	}
	second := res.(fileChunk)
	if !second.EOF || second.Length != 6 {
		t.Fatalf("unexpected second chunk: %+v", second)
	}
	if got, _ := base64.StdEncoding.DecodeString(second.Data); string(got) != " world" {
		t.Fatalf("second chunk data = %q", got)
	}
}

func TestFileDownloadRejectsUndeliveredPath(t *testing.T) {
	c, _, dir := newFileTestConnector(t)
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := c.handleFileDownload(t.Context(), &Request{Params: []byte(
		`{"session_id":"s1","path":"secret.txt"}`,
	)})
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	fe, ok := err.(*FrameError)
	if !ok || fe.Code != ErrCodeForbidden {
		t.Fatalf("error = %v, want forbidden", err)
	}
}

func TestFileDownloadRequiresSessionAndPath(t *testing.T) {
	c, _, _ := newFileTestConnector(t)

	if _, err := c.handleFileDownload(t.Context(), &Request{Params: []byte(`{"path":"a.txt"}`)}); err == nil {
		t.Fatal("expected session_id error")
	}
	if _, err := c.handleFileDownload(t.Context(), &Request{Params: []byte(`{"session_id":"s1"}`)}); err == nil {
		t.Fatal("expected path error")
	}
}

func TestRunToolResultCarriesFiles(t *testing.T) {
	c, _, dir := newFileTestConnector(t)
	path := filepath.Join(dir, "build.zip")
	if err := os.WriteFile(path, []byte("zipdata"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := &connState{out: make(chan *Frame, 8), done: make(chan struct{})}
	c.cur.Store(cs)

	r := &run{id: "r1", sessionID: "s1", state: runStateRunning}
	c.runs.handleStream(r, agent.StreamEvent{
		Type:       agent.StreamToolResult,
		ToolCall:   &llm.ToolCall{ID: "c1", Function: llm.Function{Name: "sendfile"}},
		ToolOutput: "Sent 1 file(s) to the user",
		ToolAttachments: []llm.Attachment{{
			Type:      llm.AttachmentTypeFile,
			FileName:  "build.zip",
			MediaType: "application/zip",
			Path:      path,
			Size:      7,
		}},
	})

	frame := <-cs.out
	if frame.Event != eventRunToolResult {
		t.Fatalf("event = %s", frame.Event)
	}
	var payload runToolResultPayload
	if err := jsonUnmarshal(frame.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(payload.Attachments))
	}
	att := payload.Attachments[0]
	if att.Type != llm.AttachmentTypeFile || att.Path != path || att.Size != 7 {
		t.Fatalf("attachment = %+v", att)
	}
	if att.Data != "" {
		t.Fatal("connect payload must not inline file bytes")
	}
	if _, ok := c.deliveredFile("s1", path); !ok {
		t.Fatal("delivered file should be remembered for downloads")
	}
}

func TestSessionMessageAttachmentSanitization(t *testing.T) {
	user := &session.Message{
		ID: "m1", SessionID: "s1", Role: llm.RoleUser,
		Content: llm.Content{Text: "hi", Attachments: []llm.Attachment{{
			Type: llm.AttachmentTypeImage, Data: "aGVsbG8=", MediaType: "image/png",
		}}},
	}
	out := toSessionMessage(user)
	content, ok := out.Content.(llm.Content)
	if !ok {
		t.Fatalf("user content type = %T", out.Content)
	}
	if content.Attachments[0].Data != "aGVsbG8=" {
		t.Fatal("user attachment data must be preserved for web previews")
	}

	tool := &session.Message{
		ID: "m2", SessionID: "s1", Role: llm.RoleTool, ToolCallID: "c1",
		Content: llm.Content{Text: "sent", Attachments: []llm.Attachment{{
			Type: llm.AttachmentTypeFile, Data: "c2VjcmV0", Path: "/tmp/a.zip",
		}}},
	}
	tout := toSessionMessage(tool)
	tcontent, ok := tout.Content.(llm.Content)
	if !ok {
		t.Fatalf("tool content type = %T", tout.Content)
	}
	if tcontent.Attachments[0].Data != "" {
		t.Fatal("tool attachment data must be stripped")
	}
}

func TestSanitizeMessageContentStripsData(t *testing.T) {
	content := llm.Content{
		Text: "sent",
		Attachments: []llm.Attachment{{
			Type:     llm.AttachmentTypeFile,
			Data:     "c2VjcmV0",
			FileName: "a.txt",
			Path:     "/tmp/a.txt",
			Size:     6,
		}},
	}
	out, ok := sanitizeMessageContent(content).(llm.Content)
	if !ok {
		t.Fatalf("content type = %T", out)
	}
	if out.Attachments[0].Data != "" {
		t.Fatal("inline data should be stripped")
	}
	if out.Attachments[0].Size != 6 {
		t.Fatalf("size = %d", out.Attachments[0].Size)
	}

	mapped := map[string]any{
		"text": "sent",
		"attachments": []any{
			map[string]any{"type": "file", "data": "c2VjcmV0", "file_name": "a.txt"},
		},
	}
	outMap, ok := sanitizeMessageContent(mapped).(map[string]any)
	if !ok {
		t.Fatalf("content type = %T", outMap)
	}
	atts, _ := outMap["attachments"].([]any)
	first, _ := atts[0].(map[string]any)
	if first["data"] != "" {
		t.Fatal("inline data should be stripped from history content")
	}
	if mapped["attachments"].([]any)[0].(map[string]any)["data"] != "c2VjcmV0" {
		t.Fatal("original content must not be mutated")
	}
}
