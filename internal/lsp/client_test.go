package lsp

import (
	"context"
	"sync"
	"testing"
	"time"

	json "github.com/goccy/go-json"
)

type scriptedTransport struct {
	frames chan []byte
	closed chan struct{}

	mu       sync.Mutex
	methods  []string
	onChange func(version int, text string) []map[string]any
}

func newScriptedTransport() *scriptedTransport {
	return &scriptedTransport{
		frames: make(chan []byte, 32),
		closed: make(chan struct{}),
	}
}

func scriptedRange() map[string]any {
	return map[string]any{
		"start": map[string]any{"line": 0, "character": 0},
		"end":   map[string]any{"line": 0, "character": 1},
	}
}

func (t *scriptedTransport) record(method, uri string) {
	entry := method
	if uri != "" {
		entry += " " + uri
	}
	t.mu.Lock()
	t.methods = append(t.methods, entry)
	t.mu.Unlock()
}

func (t *scriptedTransport) methodOrder() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.methods))
	copy(out, t.methods)
	return out
}

func (t *scriptedTransport) setOnChange(fn func(version int, text string) []map[string]any) {
	t.mu.Lock()
	t.onChange = fn
	t.mu.Unlock()
}

type scriptedDoc struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

func documentURI(params json.RawMessage) string {
	var p struct {
		TextDocument scriptedDoc `json:"textDocument"`
	}
	_ = json.Unmarshal(params, &p)
	return p.TextDocument.URI
}

func (t *scriptedTransport) Write(data []byte) error {
	select {
	case <-t.closed:
		return ErrClosed
	default:
	}
	var req struct {
		ID     int             `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	_ = json.Unmarshal(data, &req)
	t.record(req.Method, documentURI(req.Params))
	switch req.Method {
	case MethodInitialize:
		t.frames <- scriptedResponse(req.ID, map[string]any{
			"capabilities": map[string]any{},
			"serverInfo":   map[string]any{"name": "fake", "version": "1.0"},
		})
	case MethodShutdown:
		t.frames <- scriptedResponse(req.ID, map[string]any{})
	case MethodDidOpen:
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		_ = json.Unmarshal(req.Params, &p)
		t.frames <- scriptedNotification(MethodPublishDiagnostics, map[string]any{
			"uri":     p.TextDocument.URI,
			"version": 1,
			"diagnostics": []map[string]any{
				{
					"severity": 1,
					"message":  "fake error",
					"range":    scriptedRange(),
				},
			},
		})
	case MethodDidChange:
		var p struct {
			TextDocument   scriptedDoc `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(req.Params, &p)
		text := ""
		if len(p.ContentChanges) > 0 {
			text = p.ContentChanges[0].Text
		}
		t.mu.Lock()
		fn := t.onChange
		t.mu.Unlock()
		var diags []map[string]any
		if fn != nil {
			diags = fn(p.TextDocument.Version, text)
		}
		if diags == nil {
			diags = []map[string]any{}
		}
		t.frames <- scriptedNotification(MethodPublishDiagnostics, map[string]any{
			"uri":         p.TextDocument.URI,
			"version":     p.TextDocument.Version,
			"diagnostics": diags,
		})
	case MethodDidSave:
		// no-op
	case MethodExit:
		_ = t.Close()
	}
	return nil
}

func (t *scriptedTransport) ReadFrame() ([]byte, error) {
	select {
	case <-t.closed:
		return nil, ErrClosed
	case f := <-t.frames:
		return f, nil
	}
}

func (t *scriptedTransport) Close() error {
	select {
	case <-t.closed:
		return nil
	default:
	}
	close(t.closed)
	return nil
}

func scriptedResponse(id int, result map[string]any) []byte {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	return data
}

func scriptedNotification(method string, params any) []byte {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	return data
}

type diagCollector struct {
	mu     sync.Mutex
	uri    string
	diags  []Diagnostic
	notify chan struct{}
}

func newDiagCollector() *diagCollector {
	return &diagCollector{notify: make(chan struct{}, 4)}
}

func (d *diagCollector) onDiag(uri string, _ int, diags []Diagnostic) {
	d.mu.Lock()
	d.uri = uri
	d.diags = diags
	d.mu.Unlock()
	select {
	case d.notify <- struct{}{}:
	default:
	}
}

func (d *diagCollector) wait(t *testing.T, timeout time.Duration) {
	deadline := time.After(timeout)
	select {
	case <-d.notify:
		return
	case <-deadline:
		t.Fatal("timed out waiting for diagnostics")
	}
}

func TestClientInitializeAndDiagnostics(t *testing.T) {
	tr := newScriptedTransport()
	collector := newDiagCollector()
	c := NewClient(tr, ClientOptions{Name: "fake", Timeout: 2 * time.Second, OnDiag: collector.onDiag})
	defer c.Close()

	info, err := c.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerInfo.Name != "fake" {
		t.Fatalf("info = %+v", info)
	}

	if err := c.DidOpen(context.Background(), TextDocumentItem{
		URI:        "file:///tmp/main.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main\n",
	}); err != nil {
		t.Fatal(err)
	}

	collector.wait(t, 3*time.Second)
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.uri != "file:///tmp/main.go" {
		t.Fatalf("uri = %q", collector.uri)
	}
	if len(collector.diags) != 1 || collector.diags[0].Message != "fake error" {
		t.Fatalf("diags = %+v", collector.diags)
	}
	if collector.diags[0].Severity != 1 || collector.diags[0].Line() != 0 {
		t.Fatalf("diag = %+v", collector.diags[0])
	}
}

func TestClientSendsInitializedNotification(t *testing.T) {
	tr := newScriptedTransport()
	c := NewClient(tr, ClientOptions{Name: "fake", Timeout: 2 * time.Second})
	defer c.Close()

	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range tr.methodOrder() {
		if entry == "initialized" {
			found = true
		}
	}
	if !found {
		t.Fatalf("initialized notification must use the LSP method %q, got %v", "initialized", tr.methodOrder())
	}
}

func TestClientShutdown(t *testing.T) {
	tr := newScriptedTransport()
	collector := newDiagCollector()
	c := NewClient(tr, ClientOptions{Name: "fake", Timeout: 2 * time.Second, OnDiag: collector.onDiag})
	defer c.Close()

	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
