package attach

import (
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func TestStripBlock(t *testing.T) {
	block := Block([]llm.Attachment{{FileName: "a.txt", MediaType: "text/plain", Path: "/tmp/a.txt", Data: "eA=="}})
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello", "hello"},
		{"trailing block", "hello\n" + block, "hello"},
		{"leading block", block + "\nhello", "hello"},
		{"block only", block, ""},
		{"multiple blocks", "a" + block + "b" + block, "ab"},
		{"unterminated", "hello\n<attachments>\n<attachment/>", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripBlock(tt.in); got != tt.want {
				t.Fatalf("StripBlock(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripMessageContent(t *testing.T) {
	block := Block([]llm.Attachment{{FileName: "a.txt", MediaType: "text/plain", Path: "/tmp/a.txt"}})

	if got := StripMessageContent(llm.RoleUser, "look "+block); got != "look" {
		t.Fatalf("string content = %v", got)
	}
	if got := StripMessageContent(llm.RoleAssistant, "look "+block); got != "look "+block {
		t.Fatalf("assistant content changed: %v", got)
	}

	content := llm.Content{Text: "look " + block, Attachments: []llm.Attachment{{FileName: "a.png"}}}
	got, ok := StripMessageContent(llm.RoleUser, content).(llm.Content)
	if !ok {
		t.Fatalf("content type = %T", got)
	}
	if got.Text != "look" || len(got.Attachments) != 1 {
		t.Fatalf("content = %+v", got)
	}

	parts := []any{
		map[string]any{"type": "text", "text": "look " + block},
		map[string]any{"type": "image", "url": "x"},
	}
	stripped, ok := StripMessageContent(llm.RoleUser, parts).([]any)
	if !ok || len(stripped) != 2 {
		t.Fatalf("parts = %v", stripped)
	}
	if text := stripped[0].(map[string]any)["text"]; text != "look" {
		t.Fatalf("text part = %v", text)
	}
}
