package acp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	json "github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/utils/query"
)

type SessionNewParams struct {
	Cwd                   string      `json:"cwd"`
	MCPServers            []MCPServer `json:"mcpServers,omitempty"`
	AdditionalDirectories []string    `json:"additionalDirectories,omitempty"`
}

type SessionNewResult struct {
	SessionId      SessionId      `json:"sessionId"`
	PeggSessionId  string         `json:"peggSessionId,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	PermissionMode string         `json:"permissionMode,omitempty"`
	Commands       []SlashCommand `json:"commands,omitempty"`
}

type MCPServer struct {
	Name    string       `json:"name"`
	Command string       `json:"command,omitempty"`
	Args    []string     `json:"args,omitempty"`
	Env     []EnvVar     `json:"env,omitempty"`
	Type    string       `json:"type,omitempty"`
	URL     string       `json:"url,omitempty"`
	Headers []HttpHeader `json:"headers,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HttpHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SessionLoadParams struct {
	SessionId             SessionId   `json:"sessionId"`
	Cwd                   string      `json:"cwd"`
	MCPServers            []MCPServer `json:"mcpServers,omitempty"`
	AdditionalDirectories []string    `json:"additionalDirectories,omitempty"`
	HistoryLimit          int         `json:"historyLimit,omitempty"`
	HistoryBefore         int         `json:"historyBefore,omitempty"`
}

type SessionHistoryParams struct {
	SessionId SessionId `json:"sessionId"`
	Cwd       string    `json:"cwd,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	BeforeSeq int       `json:"beforeSeq,omitempty"`
}

type SessionHistoryResult struct {
	OldestSeq int  `json:"oldestSeq"`
	HasMore   bool `json:"hasMore"`
}

type SessionResumeParams struct {
	SessionId             SessionId   `json:"sessionId"`
	Cwd                   string      `json:"cwd"`
	MCPServers            []MCPServer `json:"mcpServers,omitempty"`
	AdditionalDirectories []string    `json:"additionalDirectories,omitempty"`
}

type SessionDeleteParams struct {
	SessionId SessionId `json:"sessionId"`
}

type SessionCloseParams struct {
	SessionId SessionId `json:"sessionId"`
}

type SessionSetModeParams struct {
	SessionId SessionId `json:"sessionId"`
	ModeId    string    `json:"modeId"`
}

type SessionListParams struct {
	Cursor string `json:"cursor,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
	All    bool   `json:"all,omitempty"`
}

type SessionListResult struct {
	Sessions   []SessionListItem `json:"sessions"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

type SessionListItem struct {
	ID         SessionId `json:"id"`
	Title      string    `json:"title"`
	ProjectDir string    `json:"projectDir,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	Mode       string    `json:"mode,omitempty"`
	Usage      llm.Usage `json:"usage,omitempty"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
}

func (s *Server) handleSessionNew(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionNewParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/new params"}
	}
	if req.Cwd == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "cwd is required"}
	}

	orch, err := s.newAgent()
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to create agent: " + err.Error()}
	}

	acpID := uuid.NewString()
	provName := ""
	if orch.Provider != nil {
		provName = orch.Provider.Name()
	}
	acpSess := &ACPSession{
		ID:        acpID,
		Agent:     orch,
		CreatedAt: time.Now(),
		Cwd:       req.Cwd,
		Provider:  provName,
		Model:     orch.Model.ID,
	}
	mode := orch.CurrentMode()
	vs, err := s.sessions.Create(session.CreateOptions{
		Title:      "ACP Session",
		Provider:   provName,
		Model:      orch.Model.ID,
		Mode:       string(mode),
		ProjectDir: req.Cwd,
	})
	if err != nil {
		s.log.Fdebug("acp: create pegg session: %v", err)
	} else {
		acpSess.PeggSessionID = vs.ID
		s.bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   orch.ID,
			SessionID: vs.ID,
		})
	}

	s.addSession(acpSess)
	s.startSessionFollow(acpSess)
	s.notifyMode(acpID, string(mode))
	s.notifyCommands(acpID)
	return SessionNewResult{
		SessionId:      SessionId(acpID),
		PeggSessionId:  acpSess.PeggSessionID,
		Mode:           string(mode),
		Provider:       acpSess.Provider,
		Model:          acpSess.Model,
		PermissionMode: s.permissionMode(),
		Commands:       availableCommands(),
	}, nil
}

func (s *Server) handleSessionLoad(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionLoadParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/load params"}
	}

	vs, err := s.sessions.Get(string(req.SessionId))
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}

	orch, err := s.newAgent()
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to create agent: " + err.Error()}
	}

	mode := agent.NormalizeMode(agent.Mode(vs.Mode))
	orch.SetMode(mode)
	s.applyStoredModel(orch, vs)

	result := SessionHistoryResult{}
	if msgs, err := s.sessionHistory(vs.ID); err == nil {
		page, oldest, hasMore := paginateHistory(msgs, req.HistoryBefore, req.HistoryLimit)
		s.emitHistoryMessages(string(req.SessionId), req.Cwd, page, false)
		result = SessionHistoryResult{OldestSeq: oldest, HasMore: hasMore}
	}
	acpSess := &ACPSession{
		ID:            string(req.SessionId),
		Agent:         orch,
		CreatedAt:     time.Now(),
		Cwd:           req.Cwd,
		Provider:      vs.Provider,
		Model:         vs.Model,
		PeggSessionID: vs.ID,
	}
	s.addSession(acpSess)

	s.bus.Publish(session.TopicSessionResume, session.SessionResume{
		AgentID:   orch.ID,
		SessionID: vs.ID,
	})

	s.notifyMode(string(req.SessionId), string(mode))
	s.notifyCommands(string(req.SessionId))
	s.startSessionFollow(acpSess)

	return result, nil
}

func (s *Server) sessionHistory(sessionID string) ([]session.Message, error) {
	msgs, err := s.sessions.Messages(sessionID)
	if err != nil {
		return nil, err
	}
	if s.runs == nil {
		return msgs, nil
	}
	active, _ := s.runs.Active(sessionID)
	if active == nil || active.BaseSeq <= 0 {
		return msgs, nil
	}
	filtered := make([]session.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Seq <= active.BaseSeq {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

func paginateHistory(msgs []session.Message, before, limit int) ([]session.Message, int, bool) {
	filtered := msgs
	if before > 0 {
		filtered = make([]session.Message, 0, len(msgs))
		for _, m := range msgs {
			if m.Seq < before {
				filtered = append(filtered, m)
			}
		}
	}
	if len(filtered) == 0 {
		return nil, 0, false
	}
	if limit <= 0 || len(filtered) <= limit {
		return filtered, filtered[0].Seq, false
	}
	start := len(filtered) - limit
	return filtered[start:], filtered[start].Seq, true
}

func (s *Server) emitHistoryMessages(sessionID, cwd string, msgs []session.Message, historyPage bool) {
	for _, msg := range msgs {
		updateType := "agent_message_chunk"
		if msg.Role == llm.RoleUser {
			updateType = "user_message_chunk"
		}
		if msg.Reasoning != nil && msg.Role == llm.RoleAssistant {
			if text, ok := msg.Reasoning.(string); ok && text != "" {
				s.notify(sessionID, SessionUpdate{
					SessionUpdate: "agent_thought_chunk",
					MessageID:     "thought-" + msg.ID,
					Content:       &ContentBlock{Type: "text", Text: text},
					Seq:           msg.Seq,
					HistoryPage:   historyPage,
				})
			}
		}
		if msg.Role == llm.RoleUser || msg.Role == llm.RoleAssistant {
			content := extractContent(msg)
			if content != nil {
				s.notify(sessionID, SessionUpdate{
					SessionUpdate: updateType,
					MessageID:     msg.ID,
					Content:       content,
					Seq:           msg.Seq,
					HistoryPage:   historyPage,
				})
			}
		}
		for _, call := range msg.ToolCalls {
			callArgs := parseArgs(call.Function.Arguments)
			s.notify(sessionID, SessionUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    call.ID,
				Title:         toolTitle(call.Function.Name, callArgs),
				Kind:          string(toolKind(call.Function.Name)),
				Status:        string(ToolCallCompleted),
				RawInput:      callArgs,
				Locations:     toolLocations(cwd, callArgs),
				Seq:           msg.Seq,
				HistoryPage:   historyPage,
			})
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			out, _ := msg.Content.(string)
			s.notify(sessionID, SessionUpdate{
				SessionUpdate: "tool_call_update",
				ToolCallID:    msg.ToolCallID,
				Status:        string(ToolCallCompleted),
				RawOutput:     truncate(out, 64*1024),
				Seq:           msg.Seq,
				HistoryPage:   historyPage,
			})
		}
	}
}

func (s *Server) handleSessionHistory(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionHistoryParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/history params"}
	}
	vs, err := s.sessions.Get(string(req.SessionId))
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	msgs, err := s.sessionHistory(vs.ID)
	if err != nil {
		return SessionHistoryResult{}, nil
	}
	page, oldest, hasMore := paginateHistory(msgs, req.BeforeSeq, req.Limit)
	s.emitHistoryMessages(string(req.SessionId), req.Cwd, page, true)
	return SessionHistoryResult{OldestSeq: oldest, HasMore: hasMore}, nil
}

func (s *Server) handleSessionResume(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionResumeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/resume params"}
	}

	vs, err := s.sessions.Get(string(req.SessionId))
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}

	orch, err := s.newAgent()
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to create agent: " + err.Error()}
	}

	mode := agent.NormalizeMode(agent.Mode(vs.Mode))
	orch.SetMode(mode)
	s.applyStoredModel(orch, vs)

	acpSess := &ACPSession{
		ID:            string(req.SessionId),
		Agent:         orch,
		CreatedAt:     time.Now(),
		Cwd:           req.Cwd,
		Provider:      vs.Provider,
		Model:         vs.Model,
		PeggSessionID: vs.ID,
	}
	s.addSession(acpSess)

	s.bus.Publish(session.TopicSessionResume, session.SessionResume{
		AgentID:   orch.ID,
		SessionID: vs.ID,
	})

	s.notifyMode(string(req.SessionId), string(mode))
	s.notifyCommands(string(req.SessionId))
	s.startSessionFollow(acpSess)

	return map[string]any{}, nil
}

func (s *Server) handleSessionDelete(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionDeleteParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/delete params"}
	}
	if s.runs != nil {
		_ = s.runs.CancelSession(string(req.SessionId))
	}
	if err := s.sessions.Delete(string(req.SessionId)); err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to delete session"}
	}
	return nil, nil
}

func (s *Server) handleSessionClose(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionCloseParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/close params"}
	}
	s.removeSession(string(req.SessionId))
	return map[string]any{}, nil
}

func (s *Server) handleSessionSetMode(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionSetModeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/set_mode params"}
	}
	acpSess, ok := s.getSession(string(req.SessionId))
	if !ok {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	m := agent.Mode(req.ModeId)
	if !m.Valid() {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid mode (want build, plan or auto)"}
	}
	m = agent.NormalizeMode(m)
	if acpSess.Agent != nil {
		acpSess.Agent.SetMode(m)
	}
	if acpSess.PeggSessionID != "" {
		_ = s.sessions.SetMode(acpSess.PeggSessionID, string(m))
	}
	s.notifyMode(acpSess.ID, string(m))
	return map[string]any{}, nil
}

func (s *Server) handleSessionList(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionListParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	size := req.Limit
	if size <= 0 || size > 200 {
		size = 50
	}
	offset := req.Offset
	if offset <= 0 && req.Cursor != "" {
		offset, _ = strconv.Atoi(req.Cursor)
	}
	if offset < 0 {
		offset = 0
	}
	filters := []query.Filter{
		session.MainSessionsFilter(),
		{Column: "compaction_parent_id", Operator: query.OpEqual, Value: ""},
	}
	if req.Cwd != "" && !req.All {
		filters = append(filters, query.Filter{Column: "project_dir", Operator: query.OpEqual, Value: req.Cwd})
	}
	q := query.Query{
		Page:    query.Page{Number: offset/size + 1, Size: size},
		Sort:    []query.Sort{{Column: "updated_at", Dir: query.Desc}},
		Filters: filters,
	}

	sessions, total, err := s.sessions.List(q)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to list sessions"}
	}

	items := make([]SessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, SessionListItem{
			ID:         SessionId(sess.ID),
			Title:      sess.Title,
			ProjectDir: sess.ProjectDir,
			Provider:   sess.Provider,
			Model:      sess.Model,
			Mode:       sess.Mode,
			Usage:      sess.Usage,
			CreatedAt:  sess.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:  sess.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	next := ""
	if offset+len(items) < total {
		next = strconv.Itoa(offset + len(items))
	}
	return SessionListResult{Sessions: items, NextCursor: next}, nil
}

func (s *Server) handleSessionRename(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		SessionId SessionId `json:"sessionId"`
		Title     string    `json:"title"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/rename params"}
	}
	id := string(req.SessionId)
	if acpSess, ok := s.getSession(string(req.SessionId)); ok && acpSess != nil && acpSess.PeggSessionID != "" {
		id = acpSess.PeggSessionID
	}
	if err := s.sessions.SetTitle(id, req.Title); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: err.Error()}
	}
	return map[string]any{}, nil
}

func (s *Server) permissionMode() string {
	if s.cfg != nil && s.cfg.Permission != nil {
		return s.cfg.Permission.Default
	}
	return "semi-ask"
}

func availableCommands() []SlashCommand {
	skills := skill.List()
	out := make([]SlashCommand, 0, len(skills))
	for _, sk := range skills {
		desc := sk.Description
		if desc == "" {
			desc = sk.WhenToUse
		}
		out = append(out, SlashCommand{Name: sk.Name, Description: desc})
	}
	return out
}

func (s *Server) notifyCommands(sessionID string) {
	s.notify(sessionID, SessionUpdate{
		SessionUpdate: "available_commands_update",
		Commands:      availableCommands(),
	})
}

func (s *Server) applyStoredModel(orch *agent.Agent, vs *session.Session) {
	if orch == nil || vs == nil || vs.Provider == "" || vs.Model == "" {
		return
	}
	prov, mdl, err := s.resolveModel(vs.Provider, vs.Model)
	if err != nil {
		s.log.Fdebug("acp: resolve stored model %s/%s: %v", vs.Provider, vs.Model, err)
		return
	}
	orch.SetModelProvider(mdl, prov)
	if vs.ReasoningEffort != "" {
		orch.ReasoningEffort = vs.ReasoningEffort
	}
}

func (s *Server) newAgent() (*agent.Agent, error) {
	return s.newAgentFn()
}

func (s *Server) newOrchestratorAgent() (*agent.Agent, error) {
	orch, err := agents.New("orchestrator")
	if err != nil {
		return nil, fmt.Errorf("create orchestrator: %w", err)
	}
	orch.Bus = s.bus

	prov, mdl, err := s.resolveModel("", "")
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}
	orch.SetModelProvider(mdl, prov)
	return orch, nil
}

func extractContent(msg session.Message) *ContentBlock {
	block := rawContent(msg)
	if block != nil && msg.Role == llm.RoleUser {
		block.Text = attach.StripBlock(block.Text)
		if block.Text == "" {
			return nil
		}
	}
	return block
}

func rawContent(msg session.Message) *ContentBlock {
	switch c := msg.Content.(type) {
	case string:
		return &ContentBlock{Type: "text", Text: c}
	case llm.Content:
		if c.Text != "" {
			return &ContentBlock{Type: "text", Text: c.Text}
		}
	case []any:
		var parts []string
		for _, item := range c {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if text, _ := m["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return &ContentBlock{Type: "text", Text: strings.Join(parts, "\n")}
		}
	}
	return nil
}
