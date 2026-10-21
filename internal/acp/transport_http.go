package acp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

const (
	HeaderConnectionID = "Acp-Connection-Id"
	HeaderSessionID    = "Acp-Session-Id"
)

type HTTPTransport struct {
	server *Server
	mu     sync.RWMutex
	conns  map[string]*HTTPConn

	replyTargets map[any]*replyTarget
	rtMu         sync.Mutex
}

type replyTarget struct {
	conn      *HTTPConn
	sessionID string
}

type HTTPConn struct {
	ID             string
	SessionStreams map[string]*sseWriter
	connStream     *sseWriter
	ws             *websocket.Conn
	mu             sync.Mutex
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	done    bool
}

func (s *sseWriter) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return fmt.Errorf("sse stream closed")
	}
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *sseWriter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
}

func NewHTTPTransport(server *Server) *HTTPTransport {
	t := &HTTPTransport{
		server:       server,
		conns:        make(map[string]*HTTPConn),
		replyTargets: make(map[any]*replyTarget),
	}
	server.responseHook = t.routeResponse
	return t
}

func (t *HTTPTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet &&
		strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
		t.handleWebSocket(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		t.handlePost(w, r)
	case http.MethodGet:
		t.handleGetStream(w, r)
	case http.MethodDelete:
		t.handleDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (t *HTTPTransport) handlePost(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); ct != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	connID := r.Header.Get(HeaderConnectionID)
	if connID == "" {
		t.handleInitializePost(w, r)
		return
	}

	conn := t.getConn(connID)
	if conn == nil {
		http.Error(w, "unknown connection id", http.StatusNotFound)
		return
	}

	var msg Request
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}

	sessID := r.Header.Get(HeaderSessionID)
	if sessID == "" {
		sessID = extractSessionIDFromParams(msg.Params)
	}

	if msg.ID != nil {
		t.rtMu.Lock()
		t.replyTargets[msg.ID] = &replyTarget{conn: conn, sessionID: sessID}
		t.rtMu.Unlock()
	}

	go t.server.dispatch(r.Context(), msg.ID, msg.Method, msg.Params)

	w.WriteHeader(http.StatusAccepted)
}

func (t *HTTPTransport) routeResponse(id any, body json.RawMessage) error {
	t.rtMu.Lock()
	target, ok := t.replyTargets[id]
	delete(t.replyTargets, id)
	t.rtMu.Unlock()
	if !ok {
		return fmt.Errorf("acp: no reply target for request id %v", id)
	}

	var sw *sseWriter
	target.conn.mu.Lock()
	if target.sessionID != "" {
		sw = target.conn.SessionStreams[target.sessionID]
	} else {
		sw = target.conn.connStream
	}
	target.conn.mu.Unlock()

	if sw == nil {
		return fmt.Errorf("acp: no stream for reply target (session=%q)", target.sessionID)
	}
	return sw.Write(body)
}

func extractSessionIDFromParams(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return ""
	}
	return p.SessionID
}

func (t *HTTPTransport) handleInitializePost(w http.ResponseWriter, r *http.Request) {
	var msg Request
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}
	if msg.Method != "initialize" {
		http.Error(w, "must initialize first", http.StatusBadRequest)
		return
	}

	conn := t.newConn()
	t.mu.Lock()
	t.conns[conn.ID] = conn
	t.mu.Unlock()

	result, err := t.server.handleInitialize(r.Context(), msg.Params)
	if err != nil {
		t.mu.Lock()
		delete(t.conns, conn.ID)
		t.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set(HeaderConnectionID, conn.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result":  result,
	})
}

func (t *HTTPTransport) handleGetStream(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "Accept must include text/event-stream", http.StatusNotAcceptable)
		return
	}

	connID := r.Header.Get(HeaderConnectionID)
	if connID == "" {
		http.Error(w, "missing connection id", http.StatusBadRequest)
		return
	}

	conn := t.getConn(connID)
	if conn == nil {
		http.Error(w, "unknown connection", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sw := &sseWriter{w: w, flusher: flusher}

	sessID := r.Header.Get(HeaderSessionID)
	conn.mu.Lock()
	if sessID != "" {
		conn.SessionStreams[sessID] = sw
	} else {
		conn.connStream = sw
	}
	conn.mu.Unlock()

	<-r.Context().Done()
	sw.Close()
}

func (t *HTTPTransport) handleDelete(w http.ResponseWriter, r *http.Request) {
	connID := r.Header.Get(HeaderConnectionID)
	if connID == "" {
		http.Error(w, "missing connection id", http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	delete(t.conns, connID)
	t.mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
}

func (t *HTTPTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn := t.newConn()
	t.mu.Lock()
	t.conns[conn.ID] = conn
	t.mu.Unlock()

	websocket.Server{Handler: func(ws *websocket.Conn) {
		defer ws.Close()
		conn.ws = ws

		origHook := t.server.responseHook
		t.server.responseHook = func(id any, body json.RawMessage) error {
			return websocket.Message.Send(ws, string(body))
		}
		defer func() { t.server.responseHook = origHook }()

		for {
			var raw string
			if err := websocket.Message.Receive(ws, &raw); err != nil {
				return
			}
			t.server.handleFrame(r.Context(), Message{Body: json.RawMessage(raw)})
		}
	}}.ServeHTTP(w, r)
}

func (t *HTTPTransport) newConn() *HTTPConn {
	return &HTTPConn{
		ID:             uuid.NewString(),
		SessionStreams: make(map[string]*sseWriter),
	}
}

func (t *HTTPTransport) getConn(id string) *HTTPConn {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.conns[id]
}

func (t *HTTPTransport) Send(msg Message) error {
	conn := t.findConnForSession(msg.SessionId)
	if conn == nil {
		return fmt.Errorf("acp: no connection for session %q", msg.SessionId)
	}
	var sw *sseWriter
	conn.mu.Lock()
	if msg.SessionId != "" {
		sw = conn.SessionStreams[msg.SessionId]
	} else {
		sw = conn.connStream
	}
	conn.mu.Unlock()
	if sw == nil {
		return fmt.Errorf("acp: no stream for session %q", msg.SessionId)
	}
	return sw.Write(msg.Body)
}

func (t *HTTPTransport) Start(ctx context.Context) (<-chan Message, error) {
	return nil, fmt.Errorf("acp: HTTPTransport does not support Start(); use ServeHTTP")
}

func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, conn := range t.conns {
		conn.mu.Lock()
		if conn.connStream != nil {
			conn.connStream.Close()
		}
		for _, sw := range conn.SessionStreams {
			sw.Close()
		}
		conn.mu.Unlock()
	}
	t.conns = make(map[string]*HTTPConn)
	return nil
}

func (t *HTTPTransport) findConnForSession(sessionID string) *HTTPConn {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if sessionID == "" {
		return nil
	}
	for _, conn := range t.conns {
		conn.mu.Lock()
		_, ok := conn.SessionStreams[sessionID]
		conn.mu.Unlock()
		if ok {
			return conn
		}
	}
	return nil
}

var _ Transport = (*HTTPTransport)(nil)
var _ = time.Second
