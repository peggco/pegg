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
