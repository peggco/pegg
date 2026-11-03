package sendfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/vfs"
)

func setupTestVFS(t *testing.T, files map[string]string) *vfs.VFS {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := vfs.New(dir, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func executeWithSink(t *testing.T, fs *vfs.VFS, args string) (string, []llm.Attachment, error) {
	t.Helper()
	sink := tool.NewAttachmentSink()
	ctx := tool.WithAttachmentSink(t.Context(), sink)
	out, err := sendFileTool(fs).Execute(ctx, args)
	return out, sink.Attachments(), err
}

func TestSendFileDeliversFiles(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{
		"reports/summary.md": "# hello\n",
		"dist/data.bin":      strings.Repeat("x", 128),
	})

	out, atts, err := executeWithSink(t, fs, `{"files":["reports/summary.md","dist/data.bin"],"message":"Here you go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 2 {
		t.Fatalf("attachments = %d, want 2", len(atts))
	}
	if !strings.Contains(out, "Sent 2 file(s)") || !strings.Contains(out, "Here you go") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "reports/summary.md") || !strings.Contains(out, "dist/data.bin") {
		t.Fatalf("output should list workspace paths:\n%s", out)
	}

	first := atts[0]
	if first.Type != llm.AttachmentTypeFile {
		t.Errorf("type = %q, want file", first.Type)
	}
	if first.FileName != "summary.md" {
		t.Errorf("file name = %q", first.FileName)
	}
	if first.MediaType != "text/markdown" {
		t.Errorf("media type = %q", first.MediaType)
	}
	if first.Size != int64(len("# hello\n")) {
		t.Errorf("size = %d", first.Size)
	}
	if first.Data != "" {
		t.Error("sendfile must not inline file bytes")
	}
	if !filepath.IsAbs(first.Path) {
		t.Errorf("path should be absolute, got %q", first.Path)
	}
	if _, err := os.Stat(first.Path); err != nil {
		t.Errorf("attachment path should exist: %v", err)
	}
	if atts[1].MediaType != "application/octet-stream" {
		t.Errorf("binary media type = %q", atts[1].MediaType)
	}
}

func TestSendFileDeduplicates(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{"a.txt": "hello"})

	out, atts, err := executeWithSink(t, fs, `{"files":["a.txt","a.txt","./a.txt"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1", len(atts))
	}
	if !strings.Contains(out, "Sent 1 file(s)") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestSendFileMissingFile(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{})

	if _, _, err := executeWithSink(t, fs, `{"files":["missing.txt"]}`); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSendFileDirectoryRejected(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{"dir/file.txt": "hello"})

	_, _, err := executeWithSink(t, fs, `{"files":["dir"]}`)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestSendFileEmptyFileRejected(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{"empty.txt": ""})

	_, _, err := executeWithSink(t, fs, `{"files":["empty.txt"]}`)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty file error, got %v", err)
	}
}

func TestSendFileRequiresFiles(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{})

	if _, _, err := executeWithSink(t, fs, `{"files":[]}`); err == nil {
		t.Fatal("expected error for empty files list")
	}
	if _, _, err := executeWithSink(t, fs, `{}`); err == nil {
		t.Fatal("expected error for missing files")
	}
}

func TestSendFileTooManyFiles(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{"a.txt": "hello"})
	paths := make([]string, MaxFiles+1)
	for i := range paths {
		paths[i] = `"a.txt"`
	}

	_, _, err := executeWithSink(t, fs, `{"files":[`+strings.Join(paths, ",")+`]}`)
	if err == nil || !strings.Contains(err.Error(), "too many files") {
		t.Fatalf("expected too many files error, got %v", err)
	}
}

func TestSendFileOutsideWorkspaceIsPermissionError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}

	spec := sendFileTool(fs)
	_, err = spec.Execute(t.Context(), `{"files":["../secret.txt"]}`)
	if err == nil {
		t.Fatal("expected out-of-bounds error")
	}
	pa, ok := spec.(tool.PermissionAware)
	if !ok || !pa.IsPermissionError(err) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		12:            "12 B",
		1024:          "1.0 KB",
		1536:          "1.5 KB",
		5 << 20:       "5.0 MB",
		2 << 30:       "2.0 GB",
		MaxFileBytes:  "200.0 MB",
		MaxTotalBytes: "500.0 MB",
	}
	for size, want := range cases {
		if got := formatBytes(size); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", size, got, want)
		}
	}
}
