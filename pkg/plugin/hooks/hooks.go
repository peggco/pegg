package hooks

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type HookKind = pkgplugin.HookKind

const (
	HookFilter = pkgplugin.HookFilter
	HookAction = pkgplugin.HookAction
)

const defaultTimeout = 30 * time.Second

var (
	ErrNestedDispatch = errors.New("hooks: nested hook dispatch is disabled")
	ErrNoHandler      = errors.New("hooks: no handler registered")
)

type Client struct {
	Binary  string
	Timeout time.Duration
	Env     []string

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr bytes.Buffer
	binary string
}

type Outcome struct {
	Value   json.RawMessage   `json:"value"`
	Outputs []json.RawMessage `json:"outputs"`
	Errors  []string          `json:"errors"`
}

type request struct {
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Value     json.RawMessage `json:"value,omitempty"`
	TimeoutMS int64           `json:"timeout_ms,omitempty"`
}

type response struct {
	Value   json.RawMessage   `json:"value,omitempty"`
	Outputs []json.RawMessage `json:"outputs,omitempty"`
	Errors  []string          `json:"errors,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (c *Client) Apply(ctx context.Context, name string, kind HookKind, value any) (Outcome, error) {
	if os.Getenv("PEGG_HOOKS_SERVE") == "1" {
		return Outcome{}, ErrNestedDispatch
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Outcome{}, errors.New("hooks: hook name is required")
	}
	if kind == "" {
		kind = HookFilter
	}
	if kind != HookFilter && kind != HookAction {
		return Outcome{}, fmt.Errorf("hooks: hook %q: unknown kind %q", name, kind)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return Outcome{}, fmt.Errorf("hooks: encode value for hook %q: %w", name, err)
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := request{Name: name, Kind: string(kind), Value: payload, TimeoutMS: timeout.Milliseconds()}
	outcome, err := c.applyOnce(callCtx, req)
	if err == nil {
		return outcome, nil
	}
	if callCtx.Err() != nil {
		return Outcome{}, fmt.Errorf("hooks: apply %q: %w", name, callCtx.Err())
	}
	c.closeProcess()
	outcome, retryErr := c.applyOnce(callCtx, req)
	if retryErr != nil {
		return Outcome{}, fmt.Errorf("hooks: apply %q via %s: %w", name, c.binaryPath(), retryErr)
	}
	return outcome, nil
}

func (c *Client) applyOnce(ctx context.Context, req request) (Outcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureProcessLocked(); err != nil {
		return Outcome{}, err
	}
	line, err := json.Marshal(req)
	if err != nil {
		return Outcome{}, err
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		message := strings.TrimSpace(c.stderr.String())
		c.closeProcessLocked()
		if message != "" {
			return Outcome{}, fmt.Errorf("%w: %s", err, message)
		}
		return Outcome{}, err
	}

	type readResult struct {
		line []byte
		err  error
	}
	read := make(chan readResult, 1)
	go func() {
		data, err := c.stdout.ReadBytes('\n')
		read <- readResult{line: data, err: err}
	}()

	select {
	case <-ctx.Done():
		c.killProcessLocked()
		return Outcome{}, ctx.Err()
	case result := <-read:
		if result.err != nil {
			message := strings.TrimSpace(c.stderr.String())
			c.closeProcessLocked()
			if message != "" {
				return Outcome{}, fmt.Errorf("%w: %s", result.err, message)
			}
			return Outcome{}, result.err
		}
		var resp response
		if err := json.Unmarshal(result.line, &resp); err != nil {
			return Outcome{}, fmt.Errorf("decode response: %w", err)
		}
		if resp.Error != "" {
			if strings.Contains(resp.Error, "no handler registered") {
				return Outcome{}, ErrNoHandler
			}
			return Outcome{}, errors.New(resp.Error)
		}
		return Outcome{Value: resp.Value, Outputs: resp.Outputs, Errors: resp.Errors}, nil
	}
}

func (c *Client) ensureProcessLocked() error {
	if c.cmd != nil && c.stdin != nil && c.stdout != nil {
		return nil
	}
	path, err := exec.LookPath(c.binaryPath())
	if err != nil {
		return fmt.Errorf("find binary %q: %w", c.binaryPath(), err)
	}
	cmd := exec.Command(path, "hooks", "serve")
	cmd.Env = append(sanitizedEnv(c.Env), "PEGG_HOOKS_SERVE=1")
	configureProcess(cmd)
	c.stderr.Reset()
	cmd.Stderr = &c.stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReaderSize(stdout, 1<<20)
	return nil
}

func (c *Client) binaryPath() string {
	if strings.TrimSpace(c.Binary) != "" {
		return strings.TrimSpace(c.Binary)
	}
	return "pegg"
}

func (c *Client) closeProcess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeProcessLocked()
}

func (c *Client) killProcessLocked() {
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}
	if c.cmd != nil {
		killProcessTree(c.cmd)
		_ = c.cmd.Wait()
	}
	c.cmd = nil
	c.stdout = nil
}

func (c *Client) closeProcessLocked() {
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}
	if c.cmd != nil {
		done := make(chan struct{})
		go func() {
			_ = c.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			killProcessTree(c.cmd)
			<-done
		}
	}
	c.cmd = nil
	c.stdout = nil
}

func (c *Client) Close() error {
	c.closeProcess()
	return nil
}

func (c *Client) FilterInto(ctx context.Context, name string, value, out any) (Outcome, error) {
	outcome, err := c.Apply(ctx, name, HookFilter, value)
	if err != nil {
		return outcome, err
	}
	if out != nil && len(outcome.Value) > 0 {
		if err := json.Unmarshal(outcome.Value, out); err != nil {
			return outcome, fmt.Errorf("hooks: decode filter %q value: %w", name, err)
		}
	}
	return outcome, nil
}

func (c *Client) Action(ctx context.Context, name string, payload any) (Outcome, error) {
	return c.Apply(ctx, name, HookAction, payload)
}

func sanitizedEnv(env []string) []string {
	base := env
	if base == nil {
		base = os.Environ()
	}
	out := make([]string, 0, len(base))
	for _, entry := range base {
		if strings.HasPrefix(entry, "PEGG_PLUGIN=") ||
			strings.HasPrefix(entry, "PEGG_HOOKS_SERVE=") ||
			strings.HasPrefix(entry, "PLUGIN_MIN_PORT=") ||
			strings.HasPrefix(entry, "PLUGIN_MAX_PORT=") ||
			strings.HasPrefix(entry, "PLUGIN_PROTOCOL_VERSIONS=") ||
			strings.HasPrefix(entry, "PLUGIN_CLIENT_CERT=") ||
			strings.HasPrefix(entry, "PLUGIN_UNIX_SOCKET_DIR=") ||
			strings.HasPrefix(entry, "PLUGIN_UNIX_SOCKET_GROUP=") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
