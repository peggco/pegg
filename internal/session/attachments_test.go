package session

import (
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func TestNormalizeContentRestoresAttachments(t *testing.T) {
	content := map[string]any{
		"text": "hi",
		"attachments": []any{
			map[string]any{
				"type":       "image",
				"media_type": "image/png",
				"data":       "AAAA",
				"file_name":  "x.png",
				"path":       "/tmp/pegg/x.png",
			},
		},
	}
	got, ok := normalizeContent(content).(llm.Content)
	if !ok {
		t.Fatalf("normalizeContent returned %T", got)
	}
	if got.Text != "hi" || len(got.Attachments) != 1 {
		t.Fatalf("unexpected content: %+v", got)
	}
	if got.Attachments[0].FileName != "x.png" || got.Attachments[0].Path != "/tmp/pegg/x.png" {
		t.Fatalf("attachment not restored: %+v", got.Attachments[0])
	}
}

func TestNormalizeContentDropsNonMedia(t *testing.T) {
	content := map[string]any{
		"text": "see files",
		"attachments": []any{
			map[string]any{"type": "file", "media_type": "application/json", "file_name": "a.json", "path": "/tmp/pegg/a.json"},
			map[string]any{"type": "image", "media_type": "image/png", "data": "AAAA", "file_name": "x.png"},
		},
	}
	got, ok := normalizeContent(content).(llm.Content)
	if !ok {
		t.Fatalf("normalizeContent returned %T", got)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("non-media not dropped: %+v", got.Attachments)
	}
}

func TestNormalizeContentPlain(t *testing.T) {
	if got := normalizeContent("hello"); got != "hello" {
		t.Fatalf("string = %v", got)
	}
	if got := normalizeContent(map[string]any{"text": "hi"}); got != "hi" {
		t.Fatalf("map without attachments = %v", got)
	}
	if got, ok := normalizeContent(llm.Content{Text: "x"}).(llm.Content); !ok || got.Text != "x" {
		t.Fatalf("llm.Content = %v", got)
	}
}

func TestMessagesToLLMAddsToolImagesAfterToolBatch(t *testing.T) {
	img := llm.NewImageAttachmentFromBase64("image/png", "AAAA")
	img.FileName = "pixel.png"
	msgs := []Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1"}, {ID: "2"}}},
		{Role: llm.RoleTool, ToolCallID: "1", Content: llm.ContentWithAttachments("image read", []llm.Attachment{img})},
		{Role: llm.RoleTool, ToolCallID: "2", Content: "plain tool"},
		{Role: llm.RoleAssistant, Content: "done"},
	}
	got := MessagesToLLM(msgs)
	if len(got) != 5 {
		t.Fatalf("messages = %d, want 5: %+v", len(got), got)
	}
	if got[1].Role != llm.RoleTool || got[1].Content != "image read" {
		t.Fatalf("tool message = %+v", got[1])
	}
	if got[2].Role != llm.RoleTool || got[2].Content != "plain tool" {
		t.Fatalf("second tool message = %+v", got[2])
	}
	content, ok := got[3].Content.(llm.Content)
	if got[3].Role != llm.RoleUser || !ok || len(content.Attachments) != 1 {
		t.Fatalf("image message = %+v", got[3])
	}
	if content.Attachments[0].FileName != "pixel.png" || content.Attachments[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("attachment = %+v", content.Attachments[0])
	}
}
