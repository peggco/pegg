package connect

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/llm"
)

func TestPrepareAttachmentsStoresIncoming(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte("hello"))
	atts, err := prepareAttachments([]llm.Attachment{
		{FileName: "a.txt", MediaType: "text/plain", Data: data},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 1 {
		t.Fatalf("got %d attachments", len(atts))
	}
	if !attach.IsStorePath(atts[0].Path) {
		t.Fatalf("path %q not in store", atts[0].Path)
	}
	if atts[0].Type != llm.AttachmentTypeFile {
		t.Fatalf("type = %q", atts[0].Type)
	}
	if atts[0].FileName != "a.txt" {
		t.Fatalf("file name = %q", atts[0].FileName)
	}
}

func TestPrepareAttachmentsFromPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("not-a-real-png"), 0o644); err != nil {
		t.Fatal(err)
	}
	atts, err := prepareAttachments(nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 1 {
		t.Fatalf("got %d attachments", len(atts))
	}
	if atts[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("type = %q, want image", atts[0].Type)
	}
	if !attach.IsStorePath(atts[0].Path) {
		t.Fatalf("path %q not in store", atts[0].Path)
	}
}

func TestPrepareAttachmentsRejectsTraversal(t *testing.T) {
	if _, err := prepareAttachments(nil, []string{"../etc/passwd"}); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
