package acpclient

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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxInbound      = 32 << 20
	maxOutbound     = 24 << 20
	readBuffer      = 64 << 10
	writeQueueDepth = 256
)

var (
	ErrClosed      = errors.New("pegg ACP server is not running")
	ErrFrameTooBig = errors.New("ACP frame exceeds size limit")
)

type Status struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type Callbacks struct {
	OnUpdate        func(sessionID string, update json.RawMessage)
	OnNotification  func(method string, params json.RawMessage)
	OnServerRequest func(requestID, method string, params json.RawMessage)
	OnStatus        func(Status)
	ReadTextFile    func(path string) (string, error)
	Logf            func(format string, args ...any)
}

type Options struct {
	Binary  string
	Cwd     string
	Env     []string
	Version string
	Callbacks
}

type pending struct {
	ch chan response
}

type response struct {
	result json.RawMessage
	err    *RPCError
}

type Client struct {
	opts Options

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writes  chan []byte
	pending map[string]*pending
	closed  bool
	lastErr error
	started bool

	nextID atomic.Int64
	exitCh chan struct{}
}

func New(opts Options) *Client {
	return &Client{
		opts:    opts,
		writes:  make(chan []byte, writeQueueDepth),
		pending: make(map[string]*pending),
		exitCh:  make(chan struct{}),
	}
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started && !c.closed {
		c.mu.Unlock()
		return nil
	}
	if c.opts.Binary == "" {
		c.mu.Unlock()
		return errors.New("pegg binary path is empty")
	}
	c.closed = false
	c.started = true
	c.exitCh = make(chan struct{})
	c.mu.Unlock()

	cmd := exec.Command(c.opts.Binary, "serve", "--acp", "--stdio")
	cmd.Dir = c.opts.Cwd
	cmd.Env = append(os.Environ(), c.opts.Env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.startFailed(err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.startFailed(err)
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.startFailed(err)
		return err
	}
	if err := cmd.Start(); err != nil {
		c.startFailed(err)
		return err
	}

	c.mu.Lock()
	c.cmd = cmd
	c.stdin = stdin
	c.mu.Unlock()

	go c.readLoop(stdout)
	go c.logStderr(stderr)
	go c.writeLoop(stdin)
	go func() {
		werr := cmd.Wait()
		c.mu.Lock()
		wasClosed := c.closed
		if c.lastErr == nil && werr != nil {
			c.lastErr = werr
		}
		c.closed = true
		c.mu.Unlock()
		c.failAll(ErrClosed)
		close(c.exitCh)
		if !wasClosed {
			msg := "Pegg server exited"
			if werr != nil {
				msg = werr.Error()
			}
			c.status(Status{State: "exited", Message: msg})
		}
	}()

	c.status(Status{State: "connected", Message: c.opts.Version})
	return nil
}

func (c *Client) startFailed(err error) {
	c.mu.Lock()
	c.started = false
	c.closed = true
	c.lastErr = err
	c.mu.Unlock()
	c.status(Status{State: "error", Message: err.Error()})
}

func (c *Client) logStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if c.opts.Logf != nil {
			c.opts.Logf("pegg stderr: %s", line)
		}
	}
}

func (c *Client) status(s Status) {
	if c.opts.OnStatus != nil {
		c.opts.OnStatus(s)
	}
}

func (c *Client) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started && !c.closed
}

func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

func (c *Client) writeLoop(w io.Writer) {
	for data := range c.writes {
		if len(data) == 0 {
			continue
		}
		if _, err := w.Write(data); err != nil {
			c.mu.Lock()
			if c.lastErr == nil {
				c.lastErr = err
			}
			c.mu.Unlock()
			c.failAll(err)
			return
		}
	}
}

func (c *Client) readLoop(r io.Reader) {
	br := bufio.NewReaderSize(r, readBuffer)
	for {
		frame, err := readFrame(br, maxInbound)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, ErrFrameTooBig) {
				if c.opts.Logf != nil {
					c.opts.Logf("dropped oversized ACP frame")
				}
				continue
			}
			if c.opts.Logf != nil {
				c.opts.Logf("ACP read error: %v", err)
			}
			return
		}
		c.handleFrame(frame)
	}
}

func readFrame(r *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(buf)+len(chunk) > max {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = r.ReadSlice('\n')
			}
			return nil, ErrFrameTooBig
		}
		buf = append(buf, chunk...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return bytes.TrimRight(buf, "\r\n"), nil
	}
}

type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func (c *Client) handleFrame(frame []byte) {
	var env envelope
	if err := json.Unmarshal(frame, &env); err != nil {
		if c.opts.Logf != nil {
			c.opts.Logf("invalid ACP frame: %v", err)
		}
		return
	}
	hasID := len(env.ID) > 0 && string(env.ID) != "null"
	switch {
	case env.Method != "" && hasID:
		go c.handleServerRequest(env.ID, env.Method, env.Params)
	case env.Method == "session/update":
		c.handleUpdate(env.Params)
	case env.Method != "":
		if c.opts.OnNotification != nil {
			c.opts.OnNotification(env.Method, env.Params)
		}
	case hasID:
		c.handleResponse(env)
	}
}

func (c *Client) handleUpdate(params json.RawMessage) {
	var p struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	if c.opts.OnUpdate != nil && len(p.Update) > 0 {
		c.opts.OnUpdate(p.SessionID, p.Update)
	}
}

func (c *Client) handleResponse(env envelope) {
	id := normID(env.ID)
	c.mu.Lock()
	p := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if p == nil {
		return
	}
	p.ch <- response{result: env.Result, err: env.Error}
}

func (c *Client) handleServerRequest(id json.RawMessage, method string, params json.RawMessage) {
	switch method {
	case "fs/read_text_file":
		c.handleReadTextFile(id, params)
		return
	case "fs/write_text_file":
		c.respondError(id, -32601, "writeTextFile is not supported")
		return
	}
	if c.opts.OnServerRequest != nil {
		c.opts.OnServerRequest(normID(id), method, params)
		return
	}
	c.respondError(id, -32601, "unsupported client method: "+method)
}

func (c *Client) handleReadTextFile(id json.RawMessage, params json.RawMessage) {
	if c.opts.ReadTextFile == nil {
		c.respondError(id, -32601, "readTextFile is not supported")
		return
	}
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		c.respondError(id, -32602, "invalid readTextFile params")
		return
	}
	text, err := c.opts.ReadTextFile(p.Path)
	if err != nil {
		c.respondError(id, -32000, err.Error())
		return
	}
	_ = c.Respond(id, map[string]any{"content": text})
}

func (c *Client) respondError(id json.RawMessage, code int, message string) {
	_ = c.Respond(id, nil, &RPCError{Code: code, Message: message})
}

func normID(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if strings.HasPrefix(s, `"`) {
		var v string
		if json.Unmarshal(raw, &v) == nil {
			return v
		}
	}
	return s
}

func (c *Client) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.Running() {
		return nil, ErrClosed
	}
	id := c.nextID.Add(1)
	key := strconv.FormatInt(id, 10)
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxOutbound {
		return nil, ErrFrameTooBig
	}
	p := &pending{ch: make(chan response, 1)}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.pending[key] = p
	c.mu.Unlock()
	if err := c.enqueue(append(raw, '\n')); err != nil {
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case resp := <-p.ch:
		if resp.err != nil {
			return nil, resp.err
		}
		return resp.result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *Client) Notify(method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(raw) > maxOutbound {
		return ErrFrameTooBig
	}
	return c.enqueue(append(raw, '\n'))
}

func (c *Client) Respond(requestID any, result any, rpcErr ...*RPCError) error {
	msg := map[string]any{"jsonrpc": "2.0", "id": requestID, "result": result}
	if len(rpcErr) > 0 && rpcErr[0] != nil {
		msg = map[string]any{"jsonrpc": "2.0", "id": requestID, "error": rpcErr[0]}
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.enqueue(append(raw, '\n'))
}

func (c *Client) enqueue(data []byte) error {
	if !c.Running() {
		return ErrClosed
	}
	select {
	case c.writes <- data:
		return nil
	default:
		return fmt.Errorf("ACP write queue is full")
	}
}

func (c *Client) failAll(err error) {
	c.mu.Lock()
	waiting := c.pending
	c.pending = make(map[string]*pending)
	c.mu.Unlock()
	for _, p := range waiting {
		select {
		case p.ch <- response{err: &RPCError{Code: -32000, Message: err.Error()}}:
		default:
		}
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.closed = true
	stdin := c.stdin
	cmd := c.cmd
	c.mu.Unlock()

	close(c.writes)
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		<-c.exitCh
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}
