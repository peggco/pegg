package shell

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	agenttool "github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/vfs"
)

func setupTestFS(t *testing.T) *vfs.VFS {
	t.Helper()
	dir := t.TempDir()
	fs, err := vfs.New(dir, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestBashToolEcho(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "echo hello world", "description": "echo hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "hello world") {
		t.Errorf("expected 'hello world', got:\n%s", out)
	}
	if !contains(t, out, "Exit code: 0") {
		t.Errorf("expected exit code 0, got:\n%s", out)
	}
}

func TestBashToolExitCode(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "exit 42", "description": "exit with code 42"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "Exit code: 42") {
		t.Errorf("expected exit code 42, got:\n%s", out)
	}
}

func TestBashToolStderr(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "echo error message >&2", "description": "echo to stderr"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "error message") {
		t.Errorf("expected 'error message' in stderr, got:\n%s", out)
	}
}

func TestBashToolMissingCommand(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	_, err := tool.Execute(t.Context(), `{}`)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestBashToolPipe(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "echo 'one two three' | wc -w", "description": "count words"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "3") {
		t.Errorf("expected word count 3, got:\n%s", out)
	}
}

func TestBashToolStdoutAndStderr(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "echo stdout; echo stderr >&2", "description": "echo stdout and stderr"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(t, out, "stdout") {
		t.Errorf("expected stdout, got:\n%s", out)
	}
	if !contains(t, out, "stderr") {
		t.Errorf("expected stderr, got:\n%s", out)
	}
}

func TestBashToolTimeout(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	start := time.Now()
	out, err := tool.Execute(t.Context(), `{"command": "echo before timeout; sleep 5", "description": "test timeout", "timeout": 1}`)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("command ran too long: %v", elapsed)
	}
	if out != "" {
		t.Errorf("expected empty output with timeout error, got:\n%s", out)
	}
	if !errors.Is(err, ErrCommandTimeout) {
		t.Errorf("expected ErrCommandTimeout, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded classification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "timed out after 1s") {
		t.Errorf("expected timeout message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "before timeout") {
		t.Errorf("expected partial output in timeout error, got: %v", err)
	}
}

func TestBashToolTruncatesOutput(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "awk 'BEGIN { for (i = 0; i < 100000; i++) printf \"x\" }'", "description": "large output"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Exit code: 0") {
		t.Errorf("expected exit code, got:\n%s", out)
	}
	if !strings.Contains(out, "[output truncated,") {
		t.Errorf("expected truncation marker, got %d bytes", len(out))
	}
	if !strings.Contains(out, "bytes omitted]") {
		t.Errorf("expected omitted byte count, got:\n%s", out)
	}
	if len(out) > maxOutputStreamBytes+1024 {
		t.Errorf("output not bounded: %d bytes", len(out))
	}
}

func TestBashToolTruncatesTerminalRunnerOutput(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)
	ctx := WithTerminalRunner(t.Context(), largeTerminalRunner{})

	out, err := tool.Execute(ctx, `{"command": "ignored", "description": "runner output"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[output truncated,") {
		t.Errorf("expected truncation marker, got %d bytes", len(out))
	}
	if len(out) > maxOutputStreamBytes+1024 {
		t.Errorf("output not bounded: %d bytes", len(out))
	}
}

type largeTerminalRunner struct{}

func (largeTerminalRunner) RunTerminal(context.Context, string, time.Duration) (string, bool, error) {
	return strings.Repeat("y", maxOutputStreamBytes+10), true, nil
}

func TestBashToolClampsTimeoutForRunner(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)
	runner := &recordingTerminalRunner{}
	ctx := WithTerminalRunner(t.Context(), runner)

	_, err := tool.Execute(ctx, `{"command": "ignored", "description": "record timeout", "timeout": 1000000000}`)
	if err != nil {
		t.Fatal(err)
	}
	if runner.timeout != maxBashTimeout {
		t.Errorf("runner timeout = %v, want %v", runner.timeout, maxBashTimeout)
	}
}

type recordingTerminalRunner struct {
	timeout time.Duration
}

func (r *recordingTerminalRunner) RunTerminal(_ context.Context, _ string, timeout time.Duration) (string, bool, error) {
	r.timeout = timeout
	return "ok", true, nil
}

func TestNormalizeTimeout(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"zero uses default", 0, defaultBashTimeout},
		{"negative uses default", -5, defaultBashTimeout},
		{"one second", 1, time.Second},
		{"max", 600, maxBashTimeout},
		{"above max is clamped", 601, maxBashTimeout},
		{"huge value is clamped", 1_000_000_000, maxBashTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTimeout(tt.seconds); got != tt.want {
				t.Errorf("normalizeTimeout(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestCappedBuffer(t *testing.T) {
	b := newCappedBuffer(4)
	n, err := b.Write([]byte("abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("Write returned %d, want 6", n)
	}
	want := "abcd\n[output truncated, 2 bytes omitted]\n"
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestTruncateOutputKeepsValidUTF8(t *testing.T) {
	s := strings.Repeat("a", maxOutputStreamBytes-1) + "€" + "tail"

	got := truncateOutput(s)
	if !utf8.ValidString(got) {
		t.Fatal("truncated output is not valid UTF-8")
	}
	if strings.Contains(got, "€") {
		t.Fatal("truncated output kept a partial rune")
	}
	if !strings.Contains(got, "[output truncated, 7 bytes omitted]") {
		t.Fatalf("unexpected omission marker:\n%s", got)
	}
	if len(got) > maxOutputStreamBytes+128 {
		t.Fatalf("output not bounded: %d bytes", len(got))
	}
}

func TestCappedBufferKeepsValidUTF8(t *testing.T) {
	b := newCappedBuffer(5)
	if _, err := b.Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("€x")); err != nil {
		t.Fatal(err)
	}

	got := b.String()
	if !utf8.ValidString(got) {
		t.Fatalf("buffer output is not valid UTF-8: %q", got)
	}
	if !strings.HasPrefix(got, "abcd") {
		t.Fatalf("buffer output = %q, want prefix %q", got, "abcd")
	}
	if !strings.Contains(got, "4 bytes omitted") {
		t.Fatalf("unexpected omission marker: %q", got)
	}
}

func TestBashToolTruncatesStderr(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	out, err := tool.Execute(t.Context(), `{"command": "awk 'BEGIN { for (i = 0; i < 100000; i++) printf \"x\" }' >&2", "description": "large stderr"}`)
	if err != nil {
		t.Fatal(err)
	}
	idx := strings.Index(out, "stderr:")
	if idx < 0 {
		t.Fatalf("expected stderr section, got:\n%s", out)
	}
	if !strings.Contains(out[idx:], "[output truncated,") {
		t.Errorf("expected stderr truncation marker, got %d bytes", len(out))
	}
	if len(out) > maxOutputStreamBytes+1024 {
		t.Errorf("stderr output not bounded: %d bytes", len(out))
	}
}

func TestBashToolCancellation(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	out, err := tool.Execute(ctx, `{"command": "echo before cancel; sleep 5", "description": "cancel", "timeout": 30}`)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if out != "" {
		t.Errorf("expected empty output with cancellation, got:\n%s", out)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if errors.Is(err, ErrCommandTimeout) {
		t.Errorf("cancellation must not be classified as a command timeout: %v", err)
	}
	if !strings.Contains(err.Error(), "before cancel") {
		t.Errorf("expected partial output in cancellation error, got: %v", err)
	}
}

func TestBashToolParentDeadlineIsNotCommandTimeout(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	parent, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := tool.Execute(parent, `{"command": "sleep 5", "description": "parent deadline", "timeout": 30}`)
	if err == nil {
		t.Fatal("expected parent deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if errors.Is(err, ErrCommandTimeout) {
		t.Errorf("parent deadline must not be classified as a command timeout: %v", err)
	}
}

func TestBashToolRunnerTimeoutIncludesPartialOutput(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)
	ctx := WithTerminalRunner(t.Context(), timeoutTerminalRunner{})

	out, err := tool.Execute(ctx, `{"command": "ignored", "description": "runner timeout"}`)
	if err == nil {
		t.Fatal("expected runner timeout error")
	}
	if out != "" {
		t.Errorf("expected empty output with runner timeout, got:\n%s", out)
	}
	if !errors.Is(err, ErrCommandTimeout) {
		t.Errorf("expected ErrCommandTimeout, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded classification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "partial output") {
		t.Errorf("expected partial output in runner timeout error, got: %v", err)
	}
}

type timeoutTerminalRunner struct{}

func (timeoutTerminalRunner) RunTerminal(context.Context, string, time.Duration) (string, bool, error) {
	return "Exit code: 0\nstdout:\npartial output\n", true,
		fmt.Errorf("%w after 1s: %w", ErrCommandTimeout, context.DeadlineExceeded)
}

func TestBashToolStreamsOutput(t *testing.T) {
	fs := setupTestFS(t)
	tool := bashTool(fs)

	var mu sync.Mutex
	var chunks []string
	sink := agenttool.NewOutputSink(func(stream, chunk string) {
		mu.Lock()
		chunks = append(chunks, stream+":"+chunk)
		mu.Unlock()
	})
	ctx := agenttool.WithOutputSink(t.Context(), sink)

	out, err := tool.Execute(ctx, `{"command": "echo one; echo two >&2", "description": "stream output"}`)
	if err != nil {
		t.Fatal(err)
	}
	sink.Close()

	if !contains(t, out, "Exit code: 0") {
		t.Errorf("expected final exit code, got:\n%s", out)
	}
	mu.Lock()
	streamed := strings.Join(chunks, "")
	mu.Unlock()
	if !strings.Contains(streamed, "stdout:one") {
		t.Errorf("missing streamed stdout, got:\n%s", streamed)
	}
	if !strings.Contains(streamed, "stderr:two") {
		t.Errorf("missing streamed stderr, got:\n%s", streamed)
	}
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
