package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"
)

const (
	defaultStreamableTimeout = 60 * time.Second
	streamableSessionHeader  = "Mcp-Session-Id"
	streamableProtocolHeader = "MCP-Protocol-Version"
	maxErrorBodySize         = 64 << 10
	streamableReconnectMin   = 500 * time.Millisecond
	streamableReconnectMax   = 5 * time.Second
)

type StreamableHTTPOptions struct {
	URL             string
	Headers         map[string]string
	Client          *http.Client
	Timeout         time.Duration
	ProtocolVersion string
}

type StreamableHTTPTransport struct {
	url             string
	protocolVersion string
	headers         map[string]string
	client          *http.Client
	timeout         time.Duration

	mu          sync.Mutex
	sessionID   string
	lastEventID string

	readCh       chan []byte
	closeCh      chan struct{}
	closeOnce    sync.Once
	streamMu     sync.Mutex
	streaming    bool
	streamCancel context.CancelFunc
}

func NewStreamableHTTPTransport(opts StreamableHTTPOptions) (*StreamableHTTPTransport, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return nil, fmt.Errorf("mcp: streamable http url is empty")
	}
	if opts.Client == nil {
		opts.Client = &http.Client{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultStreamableTimeout
	}
	if opts.ProtocolVersion == "" {
		opts.ProtocolVersion = ProtocolVersionStreamable
	}
	headers := make(map[string]string, len(opts.Headers))
	for k, v := range opts.Headers {
		headers[k] = v
	}
	return &StreamableHTTPTransport{
		url:             opts.URL,
		protocolVersion: opts.ProtocolVersion,
		headers:         headers,
		client:          opts.Client,
		timeout:         opts.Timeout,
		readCh:          make(chan []byte, 32),
		closeCh:         make(chan struct{}),
	}, nil
}

func (t *StreamableHTTPTransport) Connect(ctx context.Context) error {
	u, err := url.Parse(t.url)
	if err != nil {
		return fmt.Errorf("mcp: streamable http url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: streamable http requires http(s) url, got %q", ErrUnsupported, t.url)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: streamable http url missing host", ErrUnsupported)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (t *StreamableHTTPTransport) Write(data []byte) error {
	select {
	case <-t.closeCh:
		return ErrClosed
	default:
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	streamed := false
	defer func() {
		if !streamed {
			cancel()
		}
	}()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-t.closeCh:
			cancel()
		case <-stop:
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("mcp: streamable http build request: %w", err)
	}
	t.setHeaders(req, "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		select {
		case <-t.closeCh:
			return ErrClosed
		default:
		}
		return fmt.Errorf("mcp: streamable http post: %w", err)
	}

	t.mu.Lock()
	hadSession := t.sessionID != ""
	t.mu.Unlock()
	t.captureSession(resp)

	switch {
	case resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent:
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxFrameSize+1))
		return nil
	case resp.StatusCode == http.StatusNotFound:
		defer resp.Body.Close()
		if hadSession {
			t.mu.Lock()
			t.sessionID = ""
			t.mu.Unlock()
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodySize))
			return ErrSessionExpired
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return fmt.Errorf("%w: HTTP %d: %s", ErrProtocol, resp.StatusCode, strings.TrimSpace(string(body)))
	case resp.StatusCode >= 400:
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return fmt.Errorf("%w: HTTP %d: %s", ErrProtocol, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if responseMediaType(resp) == "text/event-stream" {
		streamed = true
		t.consumePOSTStream(resp.Body, cancel)
		return nil
	}
	defer resp.Body.Close()
	return t.handleResponse(resp)
}

func (t *StreamableHTTPTransport) ReadFrame() ([]byte, error) {
	select {
	case data := <-t.readCh:
		return data, nil
	case <-t.closeCh:
		return nil, ErrClosed
	}
}

func (t *StreamableHTTPTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closeCh)

		t.streamMu.Lock()
		cancel := t.streamCancel
		t.streamMu.Unlock()
		if cancel != nil {
			cancel()
		}

		t.mu.Lock()
		session := t.sessionID
		t.mu.Unlock()
		if session == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.url, nil)
		if err != nil {
			return
		}
		t.setHeaders(req, "")
		resp, err := t.client.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	})
	return nil
}

func (t *StreamableHTTPTransport) SetProtocolVersion(version string) {
	if version == "" {
		return
	}
	t.mu.Lock()
	t.protocolVersion = version
	t.mu.Unlock()
}

func (t *StreamableHTTPTransport) StartStream() {
	select {
	case <-t.closeCh:
		return
	default:
	}

	t.mu.Lock()
	hasSession := t.sessionID != ""
	t.mu.Unlock()
	if !hasSession {
		return
	}

	t.streamMu.Lock()
	if t.streaming {
		t.streamMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.streaming = true
	t.streamCancel = cancel
	t.streamMu.Unlock()

	go t.streamLoop(ctx)
}

func (t *StreamableHTTPTransport) streamLoop(ctx context.Context) {
	defer func() {
		t.streamMu.Lock()
		t.streaming = false
		t.streamCancel = nil
		t.streamMu.Unlock()
	}()

	backoff := streamableReconnectMin
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.closeCh:
			return
		default:
		}

		if !t.streamOnce(ctx) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-t.closeCh:
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > streamableReconnectMax {
			backoff = streamableReconnectMax
		}
	}
}

func (t *StreamableHTTPTransport) streamOnce(ctx context.Context) bool {
	t.mu.Lock()
	session := t.sessionID
	lastEventID := t.lastEventID
	t.mu.Unlock()
	if session == "" {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return false
	}
	t.setHeaders(req, "")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		return true
	}
	defer resp.Body.Close()

	t.captureSession(resp)

	switch {
	case resp.StatusCode == http.StatusMethodNotAllowed:
		return false
	case resp.StatusCode == http.StatusNotFound:
		t.mu.Lock()
		t.sessionID = ""
		t.mu.Unlock()
		return false
	case resp.StatusCode >= 400:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodySize))
		return false
	}

	err = t.consumeSSE(resp.Body)
	if ctx.Err() != nil {
		return false
	}
	return err != nil
}

func (t *StreamableHTTPTransport) handleResponse(resp *http.Response) error {
	mediaType := ""
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
			mediaType = parsed
		}
	}

	switch mediaType {
	case "application/json":
		return t.handleJSON(resp.Body)
	case "text/event-stream":
		err := t.consumeSSE(resp.Body)
		if err != nil && !errors.Is(err, io.EOF) {
			select {
			case <-t.closeCh:
				return ErrClosed
			default:
			}
			return fmt.Errorf("mcp: streamable http event stream: %w", err)
		}
		return nil
	default:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxFrameSize+1))
		return nil
	}
}

func (t *StreamableHTTPTransport) handleJSON(body io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(body, maxFrameSize+1))
	if err != nil {
		return fmt.Errorf("mcp: streamable http read body: %w", err)
	}
	if len(data) > maxFrameSize {
		return fmt.Errorf("%w: response exceeds %d bytes", ErrProtocol, maxFrameSize)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return fmt.Errorf("%w: invalid json batch: %v", ErrProtocol, err)
		}
		for _, frame := range batch {
			t.push(frame)
		}
		return nil
	}
	t.push(trimmed)
	return nil
}

func (t *StreamableHTTPTransport) consumeSSE(body io.Reader) error {
	rd := newSSELineReader(body)
	event := ""
	var data []string

	emit := func() {
		if len(data) > 0 && (event == "" || event == "message") {
			t.push([]byte(strings.Join(data, "\n")))
		}
		event = ""
		data = nil
	}

	for {
		line, err := rd.Next()
		if err != nil {
			if len(data) > 0 {
				emit()
			}
			return err
		}
		if len(line) == 0 {
			emit()
			continue
		}
		if line[0] == ':' {
			continue
		}
		field, value := parseSSE(line)
		switch field {
		case "event":
			event = value
		case "data":
			data = append(data, value)
		case "id":
			t.mu.Lock()
			t.lastEventID = value
			t.mu.Unlock()
		}
	}
}

func (t *StreamableHTTPTransport) push(frame []byte) {
	if len(frame) == 0 {
		return
	}
	cp := append([]byte(nil), frame...)
	select {
	case t.readCh <- cp:
	case <-t.closeCh:
	}
}

func (t *StreamableHTTPTransport) captureSession(resp *http.Response) {
	if session := resp.Header.Get(streamableSessionHeader); session != "" {
		t.mu.Lock()
		t.sessionID = session
		t.mu.Unlock()
	}
}

func (t *StreamableHTTPTransport) setHeaders(req *http.Request, contentType string) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	t.mu.Lock()
	session := t.sessionID
	version := t.protocolVersion
	t.mu.Unlock()
	if session != "" {
		req.Header.Set(streamableSessionHeader, session)
	}
	if version != "" {
		req.Header.Set(streamableProtocolHeader, version)
	}
}

func responseMediaType(resp *http.Response) string {
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return parsed
}

func (t *StreamableHTTPTransport) consumePOSTStream(body io.ReadCloser, cancel context.CancelFunc) {
	go func() {
		defer body.Close()
		defer cancel()

		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-t.closeCh:
				cancel()
			case <-done:
			}
		}()

		_ = t.consumeSSE(body)
	}()
}
