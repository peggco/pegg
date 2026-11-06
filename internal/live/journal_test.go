package live

import (
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func TestDecodeAgentEventToolAttachments(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"call_id":   "c1",
		"tool_name": "sendfile",
		"output":    "Sent 1 file(s) to the user",
		"attachments": []llm.Attachment{{
			Type:      llm.AttachmentTypeFile,
			FileName:  "build.zip",
			MediaType: "application/zip",
			Path:      "/tmp/build.zip",
			Size:      2048,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := DecodeAgentEvent(session.RunEvent{
		Type:    session.AgentEventToolResult,
		AgentID: "a1",
		Data:    data,
	})
	if !ok {
		t.Fatal("expected the event to decode")
	}
	if len(ev.ToolAttachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(ev.ToolAttachments))
	}
	att := ev.ToolAttachments[0]
	if att.FileName != "build.zip" || att.Path != "/tmp/build.zip" || att.Size != 2048 {
		t.Fatalf("attachment = %+v", att)
	}
	if att.Type != llm.AttachmentTypeFile {
		t.Fatalf("type = %q", att.Type)
	}
}

func TestDecodeAgentEventToolDuration(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"call_id":     "c1",
		"tool_name":   "bash",
		"output":      "ok",
		"duration_ms": int64(1500),
	})
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := DecodeAgentEvent(session.RunEvent{
		Type:    session.AgentEventToolResult,
		AgentID: "a1",
		Data:    data,
	})
	if !ok {
		t.Fatal("expected the event to decode")
	}
	if ev.ToolDuration != 1500*time.Millisecond {
		t.Fatalf("duration = %v, want 1.5s", ev.ToolDuration)
	}
}
