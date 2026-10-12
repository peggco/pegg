package attach

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/llm"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pegg-attach-test")
	if err != nil {
		panic(err)
	}
	for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
		_ = os.Setenv(key, dir)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestKind(t *testing.T) {
	cases := map[string]llm.AttachmentType{
		"image/png":       llm.AttachmentTypeImage,
		"audio/mpeg":      llm.AttachmentTypeAudio,
		"video/mp4":       llm.AttachmentTypeVideo,
		"application/pdf": llm.AttachmentTypeFile,
		"text/plain":      llm.AttachmentTypeFile,
	}
	for mediaType, want := range cases {
		if got := Kind(mediaType); got != want {
			t.Errorf("Kind(%q) = %q, want %q", mediaType, got, want)
		}
	}
}

func TestFromBytesStoresFile(t *testing.T) {
	data := []byte("hello world")
	att, err := FromBytes("greeting.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	if att.Type != llm.AttachmentTypeFile {
		t.Errorf("type = %q", att.Type)
	}
	if att.FileName != "greeting.txt" {
		t.Errorf("file name = %q", att.FileName)
	}
	if att.MediaType != "text/plain" {
		t.Errorf("media type = %q", att.MediaType)
	}
	if !IsStorePath(att.Path) {
		t.Errorf("path %q is not in the store", att.Path)
	}
	onDisk, err := os.ReadFile(att.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != "hello world" {
		t.Errorf("stored = %q", onDisk)
	}
	decoded, _ := base64.StdEncoding.DecodeString(att.Data)
	if string(decoded) != "hello world" {
		t.Errorf("data = %q", decoded)
	}
}

func TestFromBase64StripsDataURL(t *testing.T) {
	payload := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("hi"))
	att, err := FromBase64("a.txt", payload)
	if err != nil {
		t.Fatal(err)
	}
	if att.Type != llm.AttachmentTypeFile || att.FileName != "a.txt" {
		t.Fatalf("unexpected attachment: %+v", att)
	}
}

func TestSanitizeName(t *testing.T) {
	att, err := FromBytes("../evil name.txt", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if att.FileName != "evil name.txt" {
		t.Errorf("display name = %q", att.FileName)
	}
	if strings.Contains(filepath.Base(att.Path), "/") || strings.Contains(att.Path, "..") {
		t.Errorf("unsafe path %q", att.Path)
	}
	if !strings.Contains(filepath.Base(att.Path), "evil_name.txt") {
		t.Errorf("sanitized path = %q", att.Path)
	}
}

func TestMediaAttachments(t *testing.T) {
	atts := []llm.Attachment{
		{Type: llm.AttachmentTypeImage, FileName: "a.png"},
		{Type: llm.AttachmentTypeAudio, FileName: "a.mp3"},
		{Type: llm.AttachmentTypeVideo, FileName: "a.mp4"},
		{Type: llm.AttachmentTypeFile, FileName: "a.json"},
		{Type: llm.AttachmentTypeFile, FileName: "a.zip"},
	}

	got := MediaAttachments(atts, llm.Model{})
	if len(got) != 3 {
		t.Fatalf("unknown modalities: got %d media attachments", len(got))
	}

	model := llm.Model{Config: &llm.ModelConfig{
		Modalities: &llm.Modalities{Input: []string{"text", "image"}},
	}}
	got = MediaAttachments(atts, model)
	if len(got) != 1 || got[0].Type != llm.AttachmentTypeImage {
		t.Fatalf("image-only model: got %+v", got)
	}
}

func TestSaveUniqueNames(t *testing.T) {
	first, err := Save("report.docx", []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save("report.docx", []byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("second save reused the same path %q", first)
	}
	if filepath.Base(first) != "report.docx" {
		t.Errorf("first base = %q, want report.docx", filepath.Base(first))
	}
	if filepath.Base(second) != "report-1.docx" {
		t.Errorf("second base = %q, want report-1.docx", filepath.Base(second))
	}
}

func TestBlock(t *testing.T) {
	att, _ := FromBytes("doc.pdf", []byte("%PDF"))
	block := Block([]llm.Attachment{att})
	if !strings.Contains(block, `name="doc.pdf"`) ||
		!strings.Contains(block, `media_type="application/pdf"`) ||
		!strings.Contains(block, `size="4"`) ||
		!strings.Contains(block, `path="`+att.Path+`"`) {
		t.Errorf("block = %q", block)
	}
	if !strings.HasPrefix(block, "<attachments>") || !strings.HasSuffix(block, "</attachments>") {
		t.Errorf("block envelope = %q", block)
	}
	if Block(nil) != "" {
		t.Error("empty block should be empty")
	}
	if Block([]llm.Attachment{{FileName: "x"}}) != "" {
		t.Error("attachments without a path should not produce a block")
	}
}

func TestBlockEscapesAttributes(t *testing.T) {
	att, _ := FromBytes("a&b.txt", []byte("x"))
	block := Block([]llm.Attachment{att})
	if !strings.Contains(block, "a&amp;b.txt") {
		t.Errorf("attributes not escaped: %q", block)
	}
}
