package agent

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func TestUserMessageFiltersNonMedia(t *testing.T) {
	a := New("test")
	a.Attachments = []llm.Attachment{
		{Type: llm.AttachmentTypeImage, MediaType: "image/png", Data: "AAAA", FileName: "x.png", Path: "/tmp/pegg/x.png"},
		{Type: llm.AttachmentTypeFile, MediaType: "application/json", Data: "BBBB", FileName: "a.json", Path: "/tmp/pegg/a.json"},
	}

	msg := a.userMessage("hi")
	content, ok := msg.Content.(llm.Content)
	if !ok {
		t.Fatalf("content = %T, want llm.Content", msg.Content)
	}
	if len(content.Attachments) != 1 || content.Attachments[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("attachments = %+v, want only the image", content.Attachments)
	}
}

func TestUserMessagePlainWithoutAttachments(t *testing.T) {
	a := New("test")
	msg := a.userMessage("hi")
	if text, ok := msg.Content.(string); !ok || text != "hi" {
		t.Fatalf("content = %#v, want the plain string", msg.Content)
	}
}

func TestMessageWithAttachmentsBlock(t *testing.T) {
	a := New("test")
	a.Attachments = []llm.Attachment{
		{Type: llm.AttachmentTypeFile, FileName: "a.json", MediaType: "application/json", Path: "/tmp/pegg/a.json"},
	}

	got := a.messageWithAttachments("look")
	if !strings.Contains(got, "look") || !strings.Contains(got, "<attachments>") || !strings.Contains(got, `name="a.json"`) {
		t.Fatalf("message = %q", got)
	}
	if !strings.HasPrefix(a.messageWithAttachments(""), "<attachments>") {
		t.Fatal("attachment-only message should start with the block")
	}

	empty := New("test")
	if empty.messageWithAttachments("plain") != "plain" {
		t.Fatal("without attachments the text should be unchanged")
	}
}
