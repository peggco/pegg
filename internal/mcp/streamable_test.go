package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	json "github.com/goccy/go-json"
)

func newTestStreamableTransport(t *testing.T, url string, opts StreamableHTTPOptions) *StreamableHTTPTransport {
	t.Helper()
	opts.URL = url
	if opts.ProtocolVersion == "" {
		opts.ProtocolVersion = ProtocolVersionStreamable
	}
	tr, err := NewStreamableHTTPTransport(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestStreamableConnectValidation(t *testing.T) {
	if _, err := NewStreamableHTTPTransport(StreamableHTTPOptions{}); err == nil {
		t.Fatal("expected error for empty url")
	}
	tr, err := NewStreamableHTTPTransport(StreamableHTTPOptions{URL: "ftp://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Connect(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	tr, err = NewStreamableHTTPTransport(StreamableHTTPOptions{URL: "https:///path"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Connect(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestStreamablePOSTJSONAndSession(t *testing.T) {
	type capture struct {
		protocol, contentType, accept string
	}
	got := make(chan capture, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		got <- capture{
			protocol:    r.Header.Get("MCP-Protocol-Version"),
			contentType: r.Header.Get("Content-Type"),
			accept:      r.Header.Get("Accept"),
		}
		w.Header().Set("Mcp-Session-Id", "sess-1")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()

	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	frame, err := tr.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if string(frame) != `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}` {
		t.Fatalf("frame = %s", string(frame))
	}
	tr.mu.Lock()
	session := tr.sessionID
	tr.mu.Unlock()
	if session != "sess-1" {
		t.Fatalf("session = %q", session)
	}

	select {
	case c := <-got:
		if c.protocol != ProtocolVersionStreamable {
			t.Fatalf("protocol header = %q", c.protocol)
		}
		if c.contentType != "application/json" {
			t.Fatalf("content type = %q", c.contentType)
		}
		if !strings.Contains(c.accept, "application/json") || !strings.Contains(c.accept, "text/event-stream") {
			t.Fatalf("accept = %q", c.accept)
		}
	case <-time.After(time.Second):
		t.Fatal("no POST received")
	}
}

func TestStreamablePOSTBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"jsonrpc":"2.0","id":1,"result":{}},{"jsonrpc":"2.0","id":2,"result":{}}]`)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"ping"}]`)); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","id":2,"result":{}}`,
	} {
		frame, err := tr.ReadFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if string(frame) != want {
			t.Fatalf("frame %d = %s, want %s", i, string(frame), want)
		}
	}
}

func TestStreamablePOSTEventStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-sse")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n")
		fmt.Fprint(w, "id: evt-1\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\"}\n\n")
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/message"}`,
	} {
		frame, err := tr.ReadFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if string(frame) != want {
			t.Fatalf("frame %d = %s, want %s", i, string(frame), want)
		}
	}
	tr.mu.Lock()
	lastID := tr.lastEventID
	tr.mu.Unlock()
	if lastID != "evt-1" {
		t.Fatalf("last event id = %q", lastID)
	}
}

func TestStreamablePOSTAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-202")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-tr.readCh:
		t.Fatalf("unexpected frame %s", string(frame))
	case <-time.After(100 * time.Millisecond):
	}
	tr.mu.Lock()
	session := tr.sessionID
	tr.mu.Unlock()
	if session != "sess-202" {
		t.Fatalf("session = %q", session)
	}
}

func TestStreamableSessionExpired(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodPost:
			n := posts.Add(1)
			if n == 1 {
				w.Header().Set("Mcp-Session-Id", "sess-1")
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}

	err := tr.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
	tr.mu.Lock()
	session := tr.sessionID
	tr.mu.Unlock()
	if session != "" {
		t.Fatalf("session not cleared: %q", session)
	}
}

func TestStreamableClientReinitializesOn404(t *testing.T) {
	var initializeCalls atomic.Int32
	var listCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			return
		}

		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		switch req.Method {
		case MethodInitialize:
			n := initializeCalls.Add(1)
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("sess-%d", n))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"protocolVersion":%q,"serverInfo":{"name":"st","version":"1"}}}`, req.ID, ProtocolVersionStreamable)
		case MethodInitialized:
			w.WriteHeader(http.StatusAccepted)
		case MethodListTools:
			if listCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"tools":[{"name":"t","inputSchema":{"type":"object"}}]}}`, req.ID)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	client := NewClient("st", tr, Options{Name: "st", ProtocolVersion: ProtocolVersionStreamable})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	specs, err := client.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Name != "t" {
		t.Fatalf("tools = %+v", specs)
	}
	if got := initializeCalls.Load(); got != 2 {
		t.Fatalf("initialize calls = %d, want 2", got)
	}
	if got := listCalls.Load(); got != 2 {
		t.Fatalf("list calls = %d, want 2", got)
	}
}

func TestStreamableGETStreamReconnect(t *testing.T) {
	var gets atomic.Int32
	lastIDs := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			n := gets.Add(1)
			lastIDs <- r.Header.Get("Last-Event-ID")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "id: evt-%d\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"n\":%d}}\n\n", n, n)
			return
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-get")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	tr.StartStream()

	for i := 0; i < 2; i++ {
		select {
		case frame := <-tr.readCh:
			if !strings.Contains(string(frame), "notifications/message") {
				t.Fatalf("frame %d = %s", i, string(frame))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for stream frame %d", i)
		}
	}

	first := <-lastIDs
	second := <-lastIDs
	if first != "" {
		t.Fatalf("first Last-Event-ID = %q", first)
	}
	if second != "evt-1" {
		t.Fatalf("reconnect Last-Event-ID = %q", second)
	}
	_ = tr.Close()
}

func TestStreamableGETMethodNotAllowed(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			return
		}
		posts.Add(1)
		w.Header().Set("Mcp-Session-Id", "sess-405")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	tr.StartStream()

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr.streamMu.Lock()
		streaming := tr.streaming
		tr.streamMu.Unlock()
		if !streaming {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream did not stop after 405")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if got := posts.Load(); got != 2 {
		t.Fatalf("posts = %d, want 2", got)
	}
}

func TestStreamableCloseDeletesSession(t *testing.T) {
	type deletion struct {
		session, protocol string
	}
	deleted := make(chan deletion, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			deleted <- deletion{
				session:  r.Header.Get("Mcp-Session-Id"),
				protocol: r.Header.Get("MCP-Protocol-Version"),
			}
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodPost:
			w.Header().Set("Mcp-Session-Id", "sess-del")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-deleted:
		if d.session != "sess-del" {
			t.Fatalf("delete session = %q", d.session)
		}
		if d.protocol != ProtocolVersionStreamable {
			t.Fatalf("delete protocol = %q", d.protocol)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DELETE not sent")
	}
}

func TestStreamableCustomHeaders(t *testing.T) {
	type request struct {
		method, auth, custom, session string
	}
	records := make(chan request, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records <- request{
			method:  r.Method,
			auth:    r.Header.Get("Authorization"),
			custom:  r.Header.Get("X-Custom"),
			session: r.Header.Get("Mcp-Session-Id"),
		}
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.Header().Set("Mcp-Session-Id", "sess-h")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
		}
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "custom-value",
		},
	})
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	tr.StartStream()

	seen := make(map[string]request)
	deadline := time.After(5 * time.Second)
	for len(seen) < 2 {
		select {
		case rec := <-records:
			seen[rec.method] = rec
		case <-deadline:
			t.Fatalf("missing requests, got %v", seen)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case rec := <-records:
		seen[rec.method] = rec
	case <-time.After(2 * time.Second):
		t.Fatal("DELETE not received")
	}

	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		rec, ok := seen[method]
		if !ok {
			t.Fatalf("%s missing", method)
		}
		if rec.auth != "Bearer token" || rec.custom != "custom-value" {
			t.Fatalf("%s headers = auth %q custom %q", method, rec.auth, rec.custom)
		}
	}
}

func TestStreamableOversizeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxFrameSize+16))
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
}

func TestStreamableBoundedErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(bytes.Repeat([]byte("e"), 4*maxErrorBodySize))
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
	if len(err.Error()) > maxErrorBodySize+128 {
		t.Fatalf("error body not bounded: %d bytes", len(err.Error()))
	}
}

func TestStreamableEmptySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()
	if err := tr.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-tr.readCh:
		t.Fatalf("unexpected frame %s", string(frame))
	case <-time.After(100 * time.Millisecond):
	}
}

func TestStreamablePOSTEventStreamReturnsBeforeClose(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "sess-live")
		fmt.Fprint(w, "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
	}))
	defer srv.Close()
	defer close(release)

	tr := newTestStreamableTransport(t, srv.URL, StreamableHTTPOptions{})
	defer tr.Close()

	done := make(chan error, 1)
	go func() {
		done <- tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write blocked until the SSE response stream closed")
	}

	frame, err := tr.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(frame), `"id":1`) {
		t.Fatalf("frame = %s", string(frame))
	}
}
