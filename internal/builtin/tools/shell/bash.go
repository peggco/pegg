package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/vfs"
)

const (
	defaultBashTimeout    = 40 * time.Second
	maxBashTimeout        = 600 * time.Second
	maxBashTimeoutSeconds = int(maxBashTimeout / time.Second)
	maxOutputStreamBytes  = 64 << 10
)

var ErrCommandTimeout = errors.New("bash: command timed out")

type cappedBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	limit   int
	omitted int
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.omitted += n
		return n, nil
	}
	if n > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.omitted += n - remaining
		return n, nil
	}
	_, _ = b.buf.Write(p)
	return n, nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := b.buf.String()
	if b.omitted == 0 {
		return out
	}
	return out + truncationMarker(b.omitted)
}

func truncationMarker(omitted int) string {
	return fmt.Sprintf("\n[output truncated, %d bytes omitted]\n", omitted)
}

func truncateOutput(s string) string {
	if len(s) <= maxOutputStreamBytes {
		return s
	}
	return s[:maxOutputStreamBytes] + truncationMarker(len(s)-maxOutputStreamBytes)
}

func normalizeTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultBashTimeout
	}
	if seconds >= maxBashTimeoutSeconds {
		return maxBashTimeout
	}
	return time.Duration(seconds) * time.Second
}

func formatBashResult(exitCode int, stdout, stderr string) string {
	result := fmt.Sprintf("Exit code: %d\n", exitCode)
	if stdout != "" {
		result += fmt.Sprintf("stdout:\n%s\n", stdout)
	}
	if stderr != "" {
		result += fmt.Sprintf("stderr:\n%s\n", stderr)
	}
	return result
}

func formatPartialOutput(stdout, stderr string) string {
	var b strings.Builder
	if stdout != "" {
		fmt.Fprintf(&b, "\nstdout:\n%s", stdout)
	}
	if stderr != "" {
		fmt.Fprintf(&b, "\nstderr:\n%s", stderr)
	}
	return b.String()
}

type TerminalRunner interface {
	RunTerminal(ctx context.Context, command string, timeout time.Duration) (output string, handled bool, err error)
}

type terminalRunnerKey struct{}

func WithTerminalRunner(ctx context.Context, r TerminalRunner) context.Context {
	return context.WithValue(ctx, terminalRunnerKey{}, r)
}

func terminalRunnerFrom(ctx context.Context) TerminalRunner {
	r, _ := ctx.Value(terminalRunnerKey{}).(TerminalRunner)
	return r
}

func generateBashToolPrompt() (string, error) {
	sys, err := bashToolPromptBuilder().
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}

func bashTool(fs *vfs.VFS) tool.Tool {
	prompt, err := generateBashToolPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to generate bash tool prompt: %v", err))
	}

	return tool.NewSpec(
		"bash",
		prompt,
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The command to execute.",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (1-600). Defaults to 40. Use a larger value for long-running commands (e.g. builds, test suites), a smaller one for quick checks.",
					"minimum":     1,
					"maximum":     600,
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Clear, concise description of what this command does in 5-10 words. Examples:\nInput: ls\nOutput: Lists files in current directory\n\nInput: git status\nOutput: Shows working tree status\n\nInput: npm install\nOutput: Installs package dependencies\n\nInput: mkdir foo\nOutput: Creates directory 'foo'",
				},
			},
			"required": []string{"command", "description"},
		},
		func(ctx context.Context, args string) (string, error) {
			var params struct {
				Command     string `json:"command"`
				Timeout     int    `json:"timeout"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("bash: invalid arguments: %w", err)
			}
			if params.Command == "" {
				return "", fmt.Errorf("bash: command is required")
			}
			if params.Description == "" {
				return "", fmt.Errorf("bash: description is required")
			}
			timeout := normalizeTimeout(params.Timeout)

			if runner := terminalRunnerFrom(ctx); runner != nil {
				if out, handled, err := runner.RunTerminal(ctx, params.Command, timeout); handled {
					if err != nil {
						return "", err
					}
					return truncateOutput(out), nil
				}
			}

			workdir := fs.Root()

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmd := exec.Command("sh", "-c", params.Command)
			cmd.Dir = workdir
			setProcessGroup(cmd)

			stdout := newCappedBuffer(maxOutputStreamBytes)
			stderr := newCappedBuffer(maxOutputStreamBytes)
			cmd.Stdout = io.MultiWriter(stdout, tool.OutputWriter(ctx, "stdout"))
			cmd.Stderr = io.MultiWriter(stderr, tool.OutputWriter(ctx, "stderr"))

			if err := cmd.Start(); err != nil {
				return "", fmt.Errorf("bash: %w", err)
			}

			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()

			var err error
			var interrupted bool
			select {
			case <-ctx.Done():
				interrupted = true
				killProcessGroup(cmd)
				select {
				case err = <-done:
				case <-time.After(2 * time.Second):
				}
			case err = <-done:
			}

			if interrupted {
				partial := formatPartialOutput(stdout.String(), stderr.String())
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return "", fmt.Errorf("%w after %s: %w%s", ErrCommandTimeout, timeout, context.DeadlineExceeded, partial)
				}
				return "", fmt.Errorf("bash: command canceled: %w%s", context.Canceled, partial)
			}

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					return "", fmt.Errorf("bash: %w", err)
				}
			}

			return formatBashResult(exitCode, stdout.String(), stderr.String()), nil
		},
	)
}
