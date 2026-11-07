package acpclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

type terminalProcess struct {
	cmd *exec.Cmd

	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
	exit      *TerminalExitStatus

	done chan struct{}
}

func (t *terminalProcess) write(p []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf.Write(p)
	if t.limit > 0 && t.buf.Len() > t.limit {
		data := t.buf.Bytes()
		keep := append([]byte(nil), data[len(data)-t.limit:]...)
		t.buf.Reset()
		t.buf.Write(keep)
		t.truncated = true
	}
}

func (t *terminalProcess) snapshot() (string, bool, *TerminalExitStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String(), t.truncated, t.exit
}

func (t *terminalProcess) setExit(status *TerminalExitStatus) {
	t.mu.Lock()
	t.exit = status
	t.mu.Unlock()
}

func (c *Client) handleTerminalCreate(id json.RawMessage, params json.RawMessage) {
	var p TerminalCreateParams
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid terminal/create params")
		return
	}
	if p.Command == "" {
		c.respondError(id, -32602, "terminal command is required")
		return
	}
	cmd := exec.Command(p.Command, p.Args...)
	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}
	env := os.Environ()
	for _, e := range p.Env {
		env = append(env, e.Name+"="+e.Value)
	}
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.respondError(id, -32603, err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.respondError(id, -32603, err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		c.respondError(id, -32603, err.Error())
		return
	}

	term := &terminalProcess{cmd: cmd, done: make(chan struct{})}
	if p.OutputByteLimit != nil && *p.OutputByteLimit > 0 {
		term.limit = *p.OutputByteLimit
	}
	go func() {
		_, _ = io.Copy(termWriter{term}, stdout)
	}()
	go func() {
		_, _ = io.Copy(termWriter{term}, stderr)
	}()
	go func() {
		werr := cmd.Wait()
		term.setExit(exitStatus(werr))
		close(term.done)
	}()

	terminalID := "term-" + strconv.FormatInt(c.termSeq.Add(1), 10)
	c.termMu.Lock()
	c.terminals[terminalID] = term
	c.termMu.Unlock()

	_ = c.Respond(id, map[string]any{"terminalId": terminalID})
}

type termWriter struct{ term *terminalProcess }

func (w termWriter) Write(p []byte) (int, error) {
	w.term.write(p)
	return len(p), nil
}

func exitStatus(err error) *TerminalExitStatus {
	status := &TerminalExitStatus{}
	if err == nil {
		code := 0
		status.ExitCode = &code
		return status
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		status.ExitCode = &code
		return status
	}
	return status
}

func (c *Client) terminal(id string) *terminalProcess {
	c.termMu.Lock()
	defer c.termMu.Unlock()
	return c.terminals[id]
}

func (c *Client) handleTerminalOutput(id json.RawMessage, params json.RawMessage) {
	var p TerminalIDParams
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid terminal/output params")
		return
	}
	term := c.terminal(p.TerminalID)
	if term == nil {
		c.respondError(id, -32002, "unknown terminal")
		return
	}
	out, truncated, exit := term.snapshot()
	_ = c.Respond(id, TerminalOutputResult{Output: out, Truncated: truncated, ExitStatus: exit})
}

func (c *Client) handleTerminalWaitForExit(id json.RawMessage, params json.RawMessage) {
	var p TerminalIDParams
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid terminal/wait_for_exit params")
		return
	}
	term := c.terminal(p.TerminalID)
	if term == nil {
		c.respondError(id, -32002, "unknown terminal")
		return
	}
	select {
	case <-term.done:
		_, _, exit := term.snapshot()
		if exit == nil {
			exit = &TerminalExitStatus{}
		}
		_ = c.Respond(id, exit)
	case <-c.exitCh:
		c.respondError(id, -32000, "ACP server is not running")
	}
}

func (c *Client) handleTerminalKill(id json.RawMessage, params json.RawMessage) {
	var p TerminalIDParams
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid terminal/kill params")
		return
	}
	term := c.terminal(p.TerminalID)
	if term == nil {
		c.respondError(id, -32002, "unknown terminal")
		return
	}
	if term.cmd.Process != nil {
		_ = term.cmd.Process.Kill()
	}
	_ = c.Respond(id, map[string]any{})
}

func (c *Client) handleTerminalRelease(id json.RawMessage, params json.RawMessage) {
	var p TerminalIDParams
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid terminal/release params")
		return
	}
	c.termMu.Lock()
	term := c.terminals[p.TerminalID]
	delete(c.terminals, p.TerminalID)
	c.termMu.Unlock()
	if term != nil && term.cmd.Process != nil {
		_ = term.cmd.Process.Kill()
	}
	_ = c.Respond(id, map[string]any{})
}

func (c *Client) closeTerminals() {
	c.termMu.Lock()
	terms := c.terminals
	c.terminals = make(map[string]*terminalProcess)
	c.termMu.Unlock()
	for _, term := range terms {
		if term.cmd.Process != nil {
			_ = term.cmd.Process.Kill()
		}
	}
}
