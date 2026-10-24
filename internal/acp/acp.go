package acp

import (
	"context"
	"net/http"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

type ACPSession struct {
	ID        string
	Agent     *agent.Agent
	CreatedAt time.Time
	Cwd       string
	Provider  string
	Model     string

	PeggSessionID string

	cancel context.CancelFunc
	ctx    context.Context
	mu     sync.Mutex

	reqID   any
	running bool

	attachCancel context.CancelFunc
	attachRunID  string
}

func (s *ACPSession) SetCancel(cancel context.CancelFunc, reqID any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	s.reqID = reqID
}

func (s *ACPSession) StartPrompt(ctx context.Context, cancel context.CancelFunc, reqID any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.ctx = ctx
	s.cancel = cancel
	s.reqID = reqID
	s.running = true
	return true
}

func (s *ACPSession) ClearPrompt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = nil
	s.running = false
}

func (s *ACPSession) PromptContext() (context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctx, s.running
}

func (s *ACPSession) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *ACPSession) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *ACPSession) SetAttach(cancel context.CancelFunc, runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attachCancel != nil {
		s.attachCancel()
	}
	s.attachCancel = cancel
	s.attachRunID = runID
}

func (s *ACPSession) ClearAttach() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attachCancel != nil {
		s.attachCancel()
	}
	s.attachCancel = nil
	s.attachRunID = ""
}

type Server struct {
	cfg      *config.Config
	bus      event.Bus
	log      *logger.Logger
	sessions *session.Manager
	llmMgr   *llm.Manager
	fs       *vfs.VFS
	runs     *run.Coordinator
	live     *live.Hub

	transport Transport
	client    *Client

	newAgentFn func() (*agent.Agent, error)

	responseHook func(id any, body json.RawMessage) error

	active map[string]*ACPSession
	agents map[string]string
	mu     sync.RWMutex

	clientCaps *ClientCapabilities

	recMu sync.Mutex
	rec   *recording

	subStreams *subagentStreamState

	askMu       sync.Mutex
	pendingAsks map[string]*pendingAsk

	closed chan struct{}
	once   sync.Once
}

func New(
	cfg *config.Config,
	bus event.Bus,
	log *logger.Logger,
	fs *vfs.VFS,
	sessions *session.Manager,
	llmMgr *llm.Manager,
	runs *run.Coordinator,
	liveHub *live.Hub,
) *Server {
	s := &Server{
		cfg:         cfg,
		bus:         bus,
		log:         log,
		sessions:    sessions,
		llmMgr:      llmMgr,
		fs:          fs,
		runs:        runs,
		live:        liveHub,
		active:      make(map[string]*ACPSession),
		agents:      make(map[string]string),
		pendingAsks: make(map[string]*pendingAsk),
		closed:      make(chan struct{}),
	}
	s.client = NewClient(s.sendRaw)
	s.newAgentFn = s.newOrchestratorAgent
	s.subscribeAsk()
	return s
}

func (s *Server) Serve(ctx context.Context, transport Transport) error {
	defer s.cleanupRecording()
	defer s.cleanupAttach()
	s.transport = transport
	inbound, err := transport.Start(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			transport.Close()
			return ctx.Err()
		case msg, ok := <-inbound:
			if !ok {
				transport.Close()
				return nil
			}
			s.handleFrame(ctx, msg)
		}
	}
}

func (s *Server) handleFrame(ctx context.Context, msg Message) {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(msg.Body, &envelope); err != nil {
		s.writeError(nil, ErrCodeParse, "parse error", nil)
		return
	}

	if envelope.JSONRPC != "2.0" {
		s.writeError(envelope.ID, ErrCodeInvalidReq, "invalid jsonrpc version", nil)
		return
	}

	if envelope.Method == "" && envelope.ID != nil {
		s.client.DeliverResponse(envelope.ID, msg.Body)
		return
	}

	if envelope.ID != nil && envelope.Method == "session/prompt" {
		go s.dispatch(ctx, envelope.ID, envelope.Method, envelope.Params)
		return
	}
	s.dispatch(ctx, envelope.ID, envelope.Method, envelope.Params)
}

func (s *Server) dispatch(ctx context.Context, id any, method string, params json.RawMessage) {
	var result any
	var handlerErr error

	switch method {
	case "initialize":
		result, handlerErr = s.handleInitialize(ctx, params)
	case "session/new":
		result, handlerErr = s.handleSessionNew(ctx, params)
	case "session/load":
		result, handlerErr = s.handleSessionLoad(ctx, params)
	case "session/resume":
		result, handlerErr = s.handleSessionResume(ctx, params)
	case "session/prompt":
		result, handlerErr = s.handleSessionPrompt(ctx, params, id)
	case "session/delete":
		result, handlerErr = s.handleSessionDelete(ctx, params)
	case "session/close":
		result, handlerErr = s.handleSessionClose(ctx, params)
	case "session/set_mode":
		result, handlerErr = s.handleSessionSetMode(ctx, params)
	case "session/set_model":
		result, handlerErr = s.handleSessionSetModel(ctx, params)
	case "session/rename":
		result, handlerErr = s.handleSessionRename(ctx, params)
	case "session/fork":
		result, handlerErr = s.handleSessionFork(ctx, params)
	case "session/revert":
		result, handlerErr = s.handleSessionRevert(ctx, params)
	case "session/list":
		result, handlerErr = s.handleSessionList(ctx, params)
	case "pegg/models":
		result, handlerErr = s.handleListModels(ctx, params)
	case "stt/transcribe":
		result, handlerErr = s.handleSTTTranscribe(ctx, params)
	case "stt/record/start":
		result, handlerErr = s.handleSTTRecordStart(ctx, params)
	case "stt/record/stop":
		result, handlerErr = s.handleSTTRecordStop(ctx, params)
	case "session/cancel":
		handlerErr = s.handleSessionCancel(ctx, params)
		if handlerErr == nil {
			return
		}
	case "$/cancelRequest":
		handlerErr = s.handleCancelRequest(ctx, params)
		if handlerErr == nil {
			return
		}
	default:
		s.writeError(id, ErrCodeMethodNotFound, "method not found: "+method, nil)
		return
	}

	if id == nil {
		return
	}

	if handlerErr != nil {
		if rpcErr, ok := handlerErr.(*RPCError); ok {
			s.writeError(id, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		} else {
			s.writeError(id, ErrCodeInternal, handlerErr.Error(), nil)
		}
		return
	}

	s.writeResult(id, result)
}

func (s *Server) sendRaw(body json.RawMessage) error {
	if s.transport == nil {
		return nil
	}
	return s.transport.Send(Message{Body: body})
}

func (s *Server) writeResult(id any, result any) {
	if id == nil {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		s.writeError(id, ErrCodeInternal, "failed to marshal result", nil)
		return
	}
	resp := Response{JSONRPC: "2.0", ID: id, Result: data}
	raw, _ := json.Marshal(resp)
	s.sendResponse(id, raw)
}

func (s *Server) writeError(id any, code int, message string, data any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message, Data: data},
	}
	raw, _ := json.Marshal(resp)
	s.sendResponse(id, raw)
}

func (s *Server) sendResponse(id any, body json.RawMessage) {
	if s.responseHook != nil {
		s.responseHook(id, body)
		return
	}
	s.sendRaw(body)
}

func (s *Server) setClientCapabilities(caps *ClientCapabilities) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientCaps = caps
}

func (s *Server) clientCapabilities() *ClientCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clientCaps
}

func (s *Server) notify(sessionID string, update SessionUpdate) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params": map[string]any{
			"sessionId": sessionID,
			"update":    update,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if s.transport != nil {
		s.transport.Send(Message{SessionId: sessionID, Body: raw})
	}
}

func (s *Server) notifyMode(sessionID, mode string) {
	s.notify(sessionID, SessionUpdate{SessionUpdate: "current_mode_update", Mode: mode})
}

func (s *Server) HTTPHandler() http.Handler {
	t := NewHTTPTransport(s)
	s.transport = t
	return t
}

func (s *Server) addSession(acpSess *ACPSession) {
	s.mu.Lock()
	old := s.active[acpSess.ID]
	s.active[acpSess.ID] = acpSess
	if acpSess.Agent != nil {
		s.agents[acpSess.Agent.ID] = acpSess.ID
	}
	s.mu.Unlock()
	if old != nil && old != acpSess {
		old.ClearAttach()
	}
}

func (s *Server) getSession(id string) (*ACPSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	acpSess, ok := s.active[id]
	return acpSess, ok
}

func (s *Server) cleanupAttach() {
	s.mu.RLock()
	sessions := make([]*ACPSession, 0, len(s.active))
	for _, acpSess := range s.active {
		sessions = append(sessions, acpSess)
	}
	s.mu.RUnlock()
	for _, acpSess := range sessions {
		acpSess.ClearAttach()
	}
}

func (s *Server) removeSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if acpSess, ok := s.active[id]; ok {
		acpSess.ClearAttach()
		acpSess.Cancel()
		for agentID, sessionID := range s.agents {
			if sessionID == id {
				delete(s.agents, agentID)
			}
		}
	}
	delete(s.active, id)
}

func (s *Server) sessionForAgent(agentID string) (*ACPSession, bool) {
	s.mu.RLock()
	id, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return s.getSession(id)
}

func (s *Server) resolveModel(provider, model string) (llm.Provider, llm.Model, error) {
	if s.llmMgr == nil {
		return nil, llm.Model{}, ErrModelUnavailable
	}
	s.llmMgr.WaitUntilReady()

	mode := llm.SelectModePreferred
	if model != "" {
		mode = llm.SelectModeExact
	}
	res := s.llmMgr.Select(llm.SelectRequest{
		Mode:     mode,
		Provider: provider,
		Model:    model,
	})
	if res.Err != nil {
		return nil, llm.Model{}, ErrModelNotFound
	}
	prov, err := s.llmMgr.Provider(res.Provider)
	if err != nil {
		return nil, llm.Model{}, ErrModelNotFound
	}
	return prov, res.Model, nil
}

type acpError struct{}

var (
	ErrModelUnavailable = &RPCError{Code: ErrCodeInternal, Message: "llm manager unavailable"}
	ErrModelNotFound    = &RPCError{Code: ErrCodeInternal, Message: "model not found"}
	_                   = acpError{}
)
