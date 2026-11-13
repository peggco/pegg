package mcp

import (
	"context"
	json "github.com/goccy/go-json"
	"sync"
	"testing"
	"time"
)

type scriptedDuplex struct {
	mu        sync.Mutex
	responses [][]byte
	written   [][]byte
	log       [][]byte
	signal    chan struct{}
	closed    chan struct{}
}

func (s *scriptedDuplex) Write(data []byte) error {
	s.mu.Lock()
	s.written = append(s.written, append([]byte(nil), data...))
	s.log = append(s.log, append([]byte(nil), data...))
	s.mu.Unlock()
	select {
	case s.signal <- struct{}{}:
	default:
	}
	return nil
}

func (s *scriptedDuplex) WrittenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.log)
}

func (s *scriptedDuplex) Written(i int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.log[i]
}

func (s *scriptedDuplex) ReadFrame() ([]byte, error) {
	for {
		s.mu.Lock()
		var req []byte
		if len(s.written) > 0 {
			req = s.written[0]
		}
		if req != nil && isNotification(req) {
			s.written = s.written[1:]
			s.mu.Unlock()
			continue
		}
		if req != nil && len(s.responses) > 0 {
			s.written = s.written[1:]
			resp := s.responses[0]
			s.responses = s.responses[1:]
			s.mu.Unlock()
			return resp, nil
		}
		s.mu.Unlock()
		select {
		case <-s.signal:
		case <-s.closed:
			return nil, ErrClosed
		}
	}
}

func (s *scriptedDuplex) Connect(_ context.Context) error {
	return nil
}

func (s *scriptedDuplex) Close() error {
	select {
	case s.closed <- struct{}{}:
	default:
	}
	return nil
}

func newScriptedDuplex(responses ...[]byte) *scriptedDuplex {
	return &scriptedDuplex{
		responses: responses,
		signal:    make(chan struct{}, 8),
		closed:    make(chan struct{}),
	}
}

func scriptedResponse(id int, result any) []byte {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	return data
}

func TestClientInitializeAndListTools(t *testing.T) {
	duplex := newScriptedDuplex(
		scriptedResponse(1, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "srv", "version": "1.0"},
		}),
		scriptedResponse(2, map[string]any{
			"tools": []map[string]any{
				{"name": "t1", "description": "d1", "inputSchema": map[string]any{"type": "object"}},
				{"name": "t2", "inputSchema": map[string]any{"type": "object"}},
			},
		}),
	)
	c := NewClient("srv", duplex, Options{Name: "srv"})
	defer c.Close()

	info, err := c.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerInfo.Name != "srv" {
		t.Fatalf("info = %+v", info)
	}

	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "t1" || tools[1].Name != "t2" {
		t.Fatalf("tools = %+v", tools)
	}

	if n := duplex.WrittenCount(); n != 3 {
		t.Fatalf("wrote %d messages, want 3 (initialize, initialized, tools/list)", n)
	}
	var n Notification
	if err := json.Unmarshal(duplex.Written(1), &n); err != nil {
		t.Fatal(err)
	}
	if n.Method != MethodInitialized {
		t.Fatalf("second message = %s, want initialized notification", string(duplex.written[1]))
	}
}

func TestClientCallTool(t *testing.T) {
	duplex := newScriptedDuplex(
		scriptedResponse(1, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "srv", "version": "1.0"},
		}),
		scriptedResponse(2, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "hello"}},
		}),
	)
	c := NewClient("srv", duplex, Options{Name: "srv"})
	defer c.Close()

	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := c.CallTool(context.Background(), "t1", map[string]any{"q": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "hello" {
		t.Fatalf("result = %+v", res)
	}

	var req Request
	if err := json.Unmarshal(duplex.Written(2), &req); err != nil {
		t.Fatal(err)
	}
	if req.Method != MethodCallTool {
		t.Fatalf("method = %s", req.Method)
	}
}

func TestClientServerError(t *testing.T) {
	duplex := newScriptedDuplex(
		scriptedResponse(1, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "srv", "version": "1.0"},
		}),
		[]byte(`{"jsonrpc":"2.0","id":2,"error":{"code":-32001,"message":"boom"}}`),
	)
	c := NewClient("srv", duplex, Options{Name: "srv"})
	defer c.Close()

	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := c.CallTool(context.Background(), "t1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok || rpcErr.Message != "boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestClientPing(t *testing.T) {
	duplex := newScriptedDuplex(
		scriptedResponse(1, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "srv", "version": "1.0"},
		}),
		scriptedResponse(2, map[string]any{}),
	)
	c := NewClient("srv", duplex, Options{Name: "srv"})
	defer c.Close()

	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClientCallBeforeInitialize(t *testing.T) {
	duplex := newScriptedDuplex()
	c := NewClient("srv", duplex, Options{Name: "srv", Timeout: 200 * time.Millisecond})
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := c.CallTool(ctx, "t1", nil)
	if err == nil {
		t.Fatal("expected error when transport never responds")
	}
}

type retryDuplex struct {
	mu           sync.Mutex
	requests     [][]byte
	responses    [][]byte
	initialized  int
	toolCalls    int
	failTools    int
	streamStarts int
	version      string
	signal       chan struct{}
	closed       chan struct{}
	closeOnce    sync.Once
}

func newRetryDuplex(failTools int) *retryDuplex {
	return &retryDuplex{
		failTools: failTools,
		signal:    make(chan struct{}, 16),
		closed:    make(chan struct{}),
	}
}

func (d *retryDuplex) Write(data []byte) error {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	d.mu.Lock()
	d.requests = append(d.requests, append([]byte(nil), data...))
	switch req.Method {
	case MethodInitialize:
		d.initialized++
	case MethodCallTool:
		d.toolCalls++
		if d.failTools > 0 {
			d.failTools--
			d.mu.Unlock()
			return ErrSessionExpired
		}
	}

	var resp []byte
	switch req.Method {
	case MethodInitialize:
		resp, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"protocolVersion": ProtocolVersionStreamable,
				"serverInfo":      map[string]any{"name": "retry", "version": "1"},
			},
		})
	case MethodCallTool:
		resp, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": "ok"}},
			},
		})
	}
	if resp != nil {
		d.responses = append(d.responses, resp)
	}
	d.mu.Unlock()

	if resp != nil {
		select {
		case d.signal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (d *retryDuplex) ReadFrame() ([]byte, error) {
	for {
		d.mu.Lock()
		if len(d.responses) > 0 {
			resp := d.responses[0]
			d.responses = d.responses[1:]
			d.mu.Unlock()
			return resp, nil
		}
		d.mu.Unlock()
		select {
		case <-d.signal:
		case <-d.closed:
			return nil, ErrClosed
		}
	}
}

func (d *retryDuplex) Connect(context.Context) error { return nil }

func (d *retryDuplex) Close() error {
	d.closeOnce.Do(func() { close(d.closed) })
	return nil
}

func (d *retryDuplex) SetProtocolVersion(version string) {
	d.mu.Lock()
	d.version = version
	d.mu.Unlock()
}

func (d *retryDuplex) StartStream() {
	d.mu.Lock()
	d.streamStarts++
	d.mu.Unlock()
}

func (d *retryDuplex) Request(i int) []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.requests[i]
}

func (d *retryDuplex) Initialized() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.initialized
}

func (d *retryDuplex) ToolCalls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.toolCalls
}

func (d *retryDuplex) StreamStarts() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.streamStarts
}

func (d *retryDuplex) Version() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.version
}

func TestClientProtocolVersionOption(t *testing.T) {
	duplex := newRetryDuplex(0)
	c := NewClient("retry", duplex, Options{Name: "retry", ProtocolVersion: ProtocolVersionStreamable})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	info, err := c.Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.ProtocolVersion != ProtocolVersionStreamable {
		t.Fatalf("protocol = %q", info.ProtocolVersion)
	}

	var req Request
	if err := json.Unmarshal(duplex.Request(0), &req); err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["protocolVersion"] != ProtocolVersionStreamable {
		t.Fatalf("sent protocol = %v", params["protocolVersion"])
	}
	if got := duplex.Version(); got != ProtocolVersionStreamable {
		t.Fatalf("transport version = %q", got)
	}
	if got := duplex.StreamStarts(); got != 1 {
		t.Fatalf("stream starts = %d", got)
	}
}

func TestClientDefaultProtocolVersion(t *testing.T) {
	duplex := newRetryDuplex(0)
	c := NewClient("retry", duplex, Options{Name: "retry"})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	var req Request
	if err := json.Unmarshal(duplex.Request(0), &req); err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["protocolVersion"] != ProtocolVersion {
		t.Fatalf("sent protocol = %v, want %q", params["protocolVersion"], ProtocolVersion)
	}
}

func TestClientSessionRetry(t *testing.T) {
	duplex := newRetryDuplex(1)
	c := NewClient("retry", duplex, Options{Name: "retry", ProtocolVersion: ProtocolVersionStreamable})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := c.CallTool(ctx, "tool", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "ok" {
		t.Fatalf("result = %+v", res)
	}
	if got := duplex.Initialized(); got != 2 {
		t.Fatalf("initialize calls = %d, want 2", got)
	}
	if got := duplex.ToolCalls(); got != 2 {
		t.Fatalf("tool calls = %d, want 2", got)
	}
}

func TestClientReinitializeCoalesces(t *testing.T) {
	duplex := newRetryDuplex(0)
	c := NewClient("retry", duplex, Options{Name: "retry", ProtocolVersion: ProtocolVersionStreamable})
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- c.reinitialize(ctx, 1)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := duplex.Initialized(); got != 2 {
		t.Fatalf("initialize calls = %d, want 2 (initial + one reinitialize)", got)
	}
}
