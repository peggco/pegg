package file

import (
	"encoding/base64"
	"os"
	"path/filepath"
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

func TestReadTool(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{
		"hello.txt": "line one\nline two\nline three\n",
	})
	tool := readTool(fs)

	out, err := tool.Execute(t.Context(), `{"filePath": "hello.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "<file>") {
		t.Errorf("expected output to contain '<file>', got:\n%s", out)
	}
	if !contains(t, out, "</file>") {
		t.Errorf("expected output to contain '</file>', got:\n%s", out)
	}
	if !contains(t, out, "line one") {
		t.Errorf("expected output to contain 'line one', got:\n%s", out)
	}
	if !contains(t, out, "line two") {
		t.Errorf("expected output to contain 'line two', got:\n%s", out)
	}
	if !contains(t, out, "line three") {
		t.Errorf("expected output to contain 'line three', got:\n%s", out)
	}
	if contains(t, out, "File has more lines") {
		t.Errorf("expected no truncation message for complete file, got:\n%s", out)
	}
}

func TestReadToolRange(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{
		"hello.txt": "line one\nline two\nline three\nline four\n",
	})
	tool := readTool(fs)

	out, err := tool.Execute(t.Context(), `{"filePath": "hello.txt", "offset": 2, "limit": 2}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "line two") {
		t.Errorf("expected output to contain 'line two', got:\n%s", out)
	}
	if !contains(t, out, "line three") {
		t.Errorf("expected output to contain 'line three', got:\n%s", out)
	}
	if contains(t, out, "line one") {
		t.Errorf("expected output NOT to contain 'line one', got:\n%s", out)
	}
	if !contains(t, out, "File has more lines") {
		t.Errorf("expected truncation message, got:\n%s", out)
	}
	if !contains(t, out, "offset") {
		t.Errorf("expected truncation message to mention 'offset', got:\n%s", out)
	}
}

func TestReadToolNotFound(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{})
	tool := readTool(fs)

	_, err := tool.Execute(t.Context(), `{"filePath": "nonexistent.txt"}`)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadToolMissingPath(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{})
	tool := readTool(fs)

	_, err := tool.Execute(t.Context(), `{}`)
	if err == nil {
		t.Fatal("expected error for missing filePath")
	}
}

func TestFormatReadOutputPlacesDiagnosticsAfterFile(t *testing.T) {
	vfsOutput := "Path: f.go | Hash: abc | Size: 10 bytes | Lines: 2\n" +
		"---\n" +
		"     1: a\n" +
		"     2: b\n" +
		"\n" +
		"<file_diagnostics>\n" +
		"f.go:0:0: boom [severity=1]\n" +
		"</file_diagnostics>\n"

	out := formatReadOutput(vfsOutput, 1, 2000)

	fileEnd := containsStr(out, "</file>")
	diagStart := containsStr(out, "<file_diagnostics>")
	if !fileEnd || !diagStart {
		t.Fatalf("expected both </file> and diagnostics, got:\n%s", out)
	}
	if idx := indexOf(out, "</file>"); idx < 0 || indexOf(out, "<file_diagnostics>") < idx {
		t.Fatalf("diagnostics must appear after </file>, got:\n%s", out)
	}
	if !contains(t, out, "boom") {
		t.Fatalf("diagnostics content missing, got:\n%s", out)
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func contains(t *testing.T, s, substr string) bool {
	t.Helper()
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestReadToolAttachesImage(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{})
	path, err := fs.Resolve("pixel.png")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	sink := tool.NewAttachmentSink()
	ctx := tool.WithAttachmentSink(t.Context(), sink)
	out, err := readTool(fs).Execute(ctx, `{"filePath":"pixel.png"}`)
	if err != nil {
		t.Fatal(err)
	}
	if contains(t, out, "<file>") || !contains(t, out, "<image>") {
		t.Fatalf("expected image output, got:\n%s", out)
	}
	atts := sink.Attachments()
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1", len(atts))
	}
	att := atts[0]
	if att.Type != llm.AttachmentTypeImage || att.MediaType != "image/png" {
		t.Fatalf("attachment = %+v", att)
	}
	if att.FileName != "pixel.png" || att.Path != "pixel.png" || att.Data == "" {
		t.Fatalf("attachment metadata = %+v", att)
	}
}

func TestReadToolDoesNotAttachSVG(t *testing.T) {
	fs := setupTestVFS(t, map[string]string{"icon.svg": "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"})
	sink := tool.NewAttachmentSink()
	out, err := readTool(fs).Execute(tool.WithAttachmentSink(t.Context(), sink), `{"filePath":"icon.svg"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "<file>") || len(sink.Attachments()) != 0 {
		t.Fatalf("SVG should be read as text: out=%q atts=%d", out, len(sink.Attachments()))
	}
}
