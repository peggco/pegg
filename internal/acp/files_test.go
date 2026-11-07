package acp

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func newPromptState(sessionID string) *promptState {
	return &promptState{
		sessionID:   sessionID,
		textIDs:     make(map[string]string),
		thoughtID:   make(map[string]string),
		oldFiles:    make(map[string]string),
		toolPending: make(map[string]map[string]bool),
	}
}

func TestToolResultFilesSurfaceAsAttachments(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt
	sess := testSession(t, srv, "s1")

	call := llm.ToolCall{ID: "c1", Function: llm.Function{Name: "sendfile"}}
	srv.handleStreamEvent(newPromptState("s1"), sess, agent.StreamEvent{
		Type:       agent.StreamToolResult,
		ToolCall:   &call,
		ToolOutput: "Sent 1 file(s) to the user",
		ToolAttachments: []llm.Attachment{{
			Type:      llm.AttachmentTypeFile,
			FileName:  "build.zip",
			MediaType: "application/zip",
			Path:      "/tmp/build.zip",
			Size:      2048,
		}},
	})

	raw := bt.last()
	for _, want := range []string{`com.pegg/attachments`, `"build.zip"`, `"application/zip"`, `"size":2048`, `"path":"/tmp/build.zip"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("tool update missing %s: %s", want, raw)
		}
	}
}

func TestToolResultMediaAttachmentsAreNotFiles(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt
	sess := testSession(t, srv, "s1")

	call := llm.ToolCall{ID: "c1", Function: llm.Function{Name: "read"}}
	srv.handleStreamEvent(newPromptState("s1"), sess, agent.StreamEvent{
		Type:       agent.StreamToolResult,
		ToolCall:   &call,
		ToolOutput: "image loaded",
		ToolAttachments: []llm.Attachment{{
			Type:      llm.AttachmentTypeImage,
			MediaType: "image/png",
			FileName:  "pixel.png",
			Data:      "aGVsbG8=",
		}},
	})

	if raw := bt.last(); strings.Contains(raw, `"attachments"`) || strings.Contains(raw, "pixel.png") {
		t.Fatalf("media attachments should not surface as delivered files: %s", raw)
	}
}

func TestHistoryToolAttachmentsSurface(t *testing.T) {
	srv := newTestServer(t)
	bt := newBufferTransport()
	srv.transport = bt

	sess, err := srv.sessions.Create(session.CreateOptions{Provider: "test", Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	msg := llm.ToolMessage("Sent 1 file(s) to the user", "c1")
	msg.Content = llm.ContentWithAttachments("Sent 1 file(s) to the user", []llm.Attachment{{
		Type:      llm.AttachmentTypeFile,
		FileName:  "report.pdf",
		MediaType: "application/pdf",
		Path:      "/tmp/report.pdf",
		Size:      4096,
	}})
	if _, err := srv.sessions.AppendMessage(sess.ID, msg); err != nil {
		t.Fatal(err)
	}
	msgs, err := srv.sessions.Messages(sess.ID)
	if err != nil {
		t.Fatal(err)
	}

	srv.emitHistoryMessages("s1", "", msgs, false)

	raw := bt.last()
	for _, want := range []string{`"report.pdf"`, `"application/pdf"`, `"size":4096`, "Sent 1 file(s) to the user"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("history update missing %s: %s", want, raw)
		}
	}
}
