package acp

import (
	"context"
	"encoding/base64"
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
	Meta                  Meta          `json:"_meta,omitempty"`
	Cwd                   string        `json:"cwd"`
	MCPServers            []MCPServer   `json:"mcpServers"`
	AdditionalDirectories []string      `json:"additionalDirectories,omitempty"`
	SystemPrompt          *SystemPrompt `json:"systemPrompt,omitempty"`
}

type SessionNewResult struct {
	Meta          Meta                  `json:"_meta,omitempty"`
	SessionId     SessionId             `json:"sessionId"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
	Models        *SessionModelState    `json:"models,omitempty"`
}

type SessionStateResult struct {
	Meta          Meta                  `json:"_meta,omitempty"`
	SessionId     SessionId             `json:"sessionId,omitempty"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
	Models        *SessionModelState    `json:"models,omitempty"`
}

type MCPServer struct {
	Meta    Meta         `json:"_meta,omitempty"`
	Name    string       `json:"name"`
	Command string       `json:"command,omitempty"`
	Args    []string     `json:"args,omitempty"`
	Env     []EnvVar     `json:"env,omitempty"`
	Type    string       `json:"type,omitempty"`
	URL     string       `json:"url,omitempty"`
	Headers []HttpHeader `json:"headers,omitempty"`
}

type EnvVar struct {
	Meta  Meta   `json:"_meta,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HttpHeader struct {
	Meta  Meta   `json:"_meta,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SessionLoadParams struct {
	Meta                  Meta          `json:"_meta,omitempty"`
	SessionId             SessionId     `json:"sessionId"`
	Cwd                   string        `json:"cwd"`
	MCPServers            []MCPServer   `json:"mcpServers"`
	AdditionalDirectories []string      `json:"additionalDirectories,omitempty"`
	SystemPrompt          *SystemPrompt `json:"systemPrompt,omitempty"`
	HistoryLimit          int           `json:"historyLimit,omitempty"`
	HistoryBefore         int           `json:"historyBefore,omitempty"`
}

type SessionHistoryParams struct {
	Meta      Meta      `json:"_meta,omitempty"`
	SessionId SessionId `json:"sessionId"`
	Cwd       string    `json:"cwd,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	BeforeSeq int       `json:"beforeSeq,omitempty"`
}

type SessionHistoryResult struct {
	Meta      Meta `json:"_meta,omitempty"`
	OldestSeq int  `json:"oldestSeq"`
	HasMore   bool `json:"hasMore"`
}

type SessionResumeParams struct {
	Meta                  Meta          `json:"_meta,omitempty"`
	SessionId             SessionId     `json:"sessionId"`
	Cwd                   string        `json:"cwd"`
	MCPServers            []MCPServer   `json:"mcpServers"`
	AdditionalDirectories []string      `json:"additionalDirectories,omitempty"`
	SystemPrompt          *SystemPrompt `json:"systemPrompt,omitempty"`
}

type SessionDeleteParams struct {
	Meta      Meta      `json:"_meta,omitempty"`
	SessionId SessionId `json:"sessionId"`
}

type SessionCloseParams struct {
	Meta      Meta      `json:"_meta,omitempty"`
	SessionId SessionId `json:"sessionId"`
}

type SessionSetModeParams struct {
	Meta      Meta      `json:"_meta,omitempty"`
	SessionId SessionId `json:"sessionId"`
	ModeId    string    `json:"modeId"`
}

type SessionSetConfigOptionParams struct {
	Meta      Meta            `json:"_meta,omitempty"`
	SessionId SessionId       `json:"sessionId"`
	ConfigId  SessionConfigId `json:"configId"`
	Type      string          `json:"type,omitempty"`
	Value     any             `json:"value"`
}

type SessionSetConfigOptionResult struct {
	Meta          Meta                  `json:"_meta,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions"`
}

type SessionListParams struct {
	Meta   Meta   `json:"_meta,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
}

type SessionListResult struct {
	Meta       Meta          `json:"_meta,omitempty"`
	Sessions   []SessionInfo `json:"sessions"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type SessionInfo struct {
	Meta                  Meta      `json:"_meta,omitempty"`
	SessionId             SessionId `json:"sessionId"`
	Cwd                   string    `json:"cwd"`
	Title                 string    `json:"title,omitempty"`
	UpdatedAt             string    `json:"updatedAt,omitempty"`
	AdditionalDirectories []string  `json:"additionalDirectories,omitempty"`
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
	if err := applySystemPrompt(orch, req.SystemPrompt); err != nil {
		return nil, err
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
		orch.SetSessionID(vs.ID)
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
		SessionId:     SessionId(acpID),
		Modes:         s.modes(orch),
		ConfigOptions: s.configOptions(orch),
		Models:        s.sessionModelState(orch),
		Meta: Meta{
			"com.pegg/sessionId":      acpSess.PeggSessionID,
			"com.pegg/permissionMode": s.permissionMode(),
		},
	}, nil
}

func (s *Server) handleSessionLoad(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionLoadParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/load params"}
	}

	vs, err := s.sessions.Get(string(req.SessionId))
	if err != nil {
		return nil, &RPCError{Code: ErrCodeResourceNotFound, Message: "session not found", Data: map[string]any{"sessionId": req.SessionId}}
	}

	orch, err := s.newAgent()
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to create agent: " + err.Error()}
	}
	if err := applySystemPrompt(orch, req.SystemPrompt); err != nil {
		return nil, err
	}

	mode := agent.NormalizeMode(agent.Mode(vs.Mode))
	orch.SetMode(mode)
	orch.SetSessionID(vs.ID)
	s.applyStoredModel(orch, vs)

	oldest, hasMore := 0, false
	if msgs, err := s.sessionHistory(vs.ID); err == nil {
		page, o, more := paginateHistory(msgs, req.HistoryBefore, req.HistoryLimit)
		s.emitHistoryMessages(string(req.SessionId), req.Cwd, page, false)
		oldest, hasMore = o, more
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

	return SessionStateResult{
		SessionId:     req.SessionId,
		Modes:         s.modes(orch),
		ConfigOptions: s.configOptions(orch),
		Models:        s.sessionModelState(orch),
		Meta: Meta{
			"com.pegg/oldestSeq": oldest,
			"com.pegg/hasMore":   hasMore,
		},
	}, nil
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
	if active == nil {
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
		meta := Meta{"com.pegg/seq": msg.Seq}
		if historyPage {
			meta["com.pegg/historyPage"] = true
		}
		if msg.Reasoning != nil && msg.Role == llm.RoleAssistant {
			if text, ok := msg.Reasoning.(string); ok && text != "" {
				s.notify(sessionID, SessionUpdate{
					SessionUpdate: "agent_thought_chunk",
					MessageID:     MessageId("thought-" + msg.ID),
					Content:       &ContentBlock{Type: "text", Text: text},
					Meta:          meta,
				})
			}
		}
		if msg.Role == llm.RoleUser || msg.Role == llm.RoleAssistant {
			content := extractContent(msg)
			var atts []AttachmentInfo
			if msg.Role == llm.RoleUser {
				atts = attachmentInfos(session.AttachmentsOf(msg))
			}
			if content != nil || len(atts) > 0 {
				if content == nil {
					content = &ContentBlock{Type: "text"}
				}
				m := Meta{"com.pegg/seq": msg.Seq}
				if historyPage {
					m["com.pegg/historyPage"] = true
				}
				if len(atts) > 0 {
					m["com.pegg/attachments"] = atts
				}
				s.notify(sessionID, SessionUpdate{
					SessionUpdate: updateType,
					MessageID:     MessageId(msg.ID),
					Content:       content,
					Meta:          m,
				})
			}
		}
		for _, call := range msg.ToolCalls {
			callArgs := parseArgs(call.Function.Arguments)
			s.notify(sessionID, SessionUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    ToolCallId(call.ID),
				Title:         toolTitle(call.Function.Name, callArgs),
				Name:          call.Function.Name,
				Kind:          toolKind(call.Function.Name),
				Status:        string(ToolCallCompleted),
				RawInput:      callArgs,
				Locations:     toolLocations(cwd, callArgs),
				Meta:          meta,
			})
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			out := ""
			if block := rawContent(msg); block != nil {
				out = block.Text
			}
			m := Meta{"com.pegg/seq": msg.Seq}
			if historyPage {
				m["com.pegg/historyPage"] = true
			}
			if atts := fileAttachments(session.AttachmentsOf(msg)); len(atts) > 0 {
				m["com.pegg/attachments"] = atts
			}
			s.notify(sessionID, SessionUpdate{
				SessionUpdate: "tool_call_update",
				ToolCallID:    ToolCallId(msg.ToolCallID),
				Status:        string(ToolCallCompleted),
				RawOutput:     truncate(out, 64*1024),
				Meta:          m,
			})
		}
	}
}

func (s *Server) handleSessionHistory(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionHistoryParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid _pegg/session/history params"}
	}
	vs, err := s.sessions.Get(string(req.SessionId))
	if err != nil {
		return nil, &RPCError{Code: ErrCodeResourceNotFound, Message: "session not found", Data: map[string]any{"sessionId": req.SessionId}}
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
		return nil, &RPCError{Code: ErrCodeResourceNotFound, Message: "session not found", Data: map[string]any{"sessionId": req.SessionId}}
	}

	orch, err := s.newAgent()
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "failed to create agent: " + err.Error()}
	}
	if err := applySystemPrompt(orch, req.SystemPrompt); err != nil {
		return nil, err
	}

	mode := agent.NormalizeMode(agent.Mode(vs.Mode))
	orch.SetMode(mode)
	orch.SetSessionID(vs.ID)
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

	return SessionStateResult{
		SessionId:     req.SessionId,
		Modes:         s.modes(orch),
		ConfigOptions: s.configOptions(orch),
		Models:        s.sessionModelState(orch),
	}, nil
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
		s.log.Fdebug("acp: delete session %s: %v", req.SessionId, err)
	}
	s.removeSession(string(req.SessionId))
	return map[string]any{}, nil
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
		return nil, &RPCError{Code: ErrCodeResourceNotFound, Message: "session not found", Data: map[string]any{"sessionId": req.SessionId}}
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
	s.notifyConfigOptions(acpSess)
	return map[string]any{}, nil
}

func (s *Server) handleSessionSetConfigOption(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionSetConfigOptionParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/set_config_option params"}
	}
	acpSess, ok := s.getSession(string(req.SessionId))
	if !ok {
		return nil, &RPCError{Code: ErrCodeResourceNotFound, Message: "session not found", Data: map[string]any{"sessionId": req.SessionId}}
	}
	value, _ := req.Value.(string)
	switch req.ConfigId {
	case "mode":
		m := agent.Mode(value)
		if !m.Valid() {
			return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid mode value"}
		}
		m = agent.NormalizeMode(m)
		if acpSess.Agent != nil {
			acpSess.Agent.SetMode(m)
		}
		if acpSess.PeggSessionID != "" {
			_ = s.sessions.SetMode(acpSess.PeggSessionID, string(m))
		}
		s.notifyMode(acpSess.ID, string(m))
	case "model":
		if acpSess.IsRunning() {
			return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "cannot change model while a prompt is running"}
		}
		prov, mdl, err := s.resolveModel("", value)
		if err != nil {
			return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "unknown model"}
		}
		if acpSess.Agent != nil {
			acpSess.Agent.SetModelProvider(mdl, prov)
		}
		acpSess.Provider = prov.Name()
		acpSess.Model = mdl.ID
		if acpSess.PeggSessionID != "" {
			_ = s.sessions.SetModel(acpSess.PeggSessionID, acpSess.Provider, acpSess.Model)
		}
	case "reasoning_effort":
		if acpSess.Agent != nil {
			acpSess.Agent.ReasoningEffort = value
		}
	default:
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "unknown configId: " + string(req.ConfigId)}
	}
	opts := s.configOptions(acpSess.Agent)
	s.notifyConfigOptions(acpSess)
	return SessionSetConfigOptionResult{ConfigOptions: opts}, nil
}

func (s *Server) handleSessionList(ctx context.Context, params json.RawMessage) (any, error) {
	var req SessionListParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	const size = 50
	offset := 0
	if req.Cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.Cursor)
		if err != nil {
			return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid cursor"}
		}
		offset, err = strconv.Atoi(string(decoded))
		if err != nil || offset < 0 {
			return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid cursor"}
		}
	}
	filters := []query.Filter{
		session.MainSessionsFilter(),
		{Column: "compaction_parent_id", Operator: query.OpEqual, Value: ""},
	}
	if req.Cwd != "" {
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

	items := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, SessionInfo{
			SessionId: SessionId(sess.ID),
			Cwd:       sess.ProjectDir,
			Title:     sess.Title,
			UpdatedAt: sess.UpdatedAt.UTC().Format(time.RFC3339),
			Meta: Meta{
				"com.pegg/provider":  sess.Provider,
				"com.pegg/model":     sess.Model,
				"com.pegg/mode":      sess.Mode,
				"com.pegg/usage":     sess.Usage,
				"com.pegg/createdAt": sess.CreatedAt.UTC().Format(time.RFC3339),
			},
		})
	}

	next := ""
	if offset+len(items) < total {
		next = base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset + len(items))))
	}
	return SessionListResult{Sessions: items, NextCursor: next}, nil
}

func (s *Server) handleSessionRename(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Meta      Meta      `json:"_meta,omitempty"`
		SessionId SessionId `json:"sessionId"`
		Title     string    `json:"title"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid _pegg/session/rename params"}
	}
	id := string(req.SessionId)
	if acpSess, ok := s.getSession(string(req.SessionId)); ok && acpSess != nil && acpSess.PeggSessionID != "" {
		id = acpSess.PeggSessionID
	}
	if err := s.sessions.SetTitle(id, req.Title); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: err.Error()}
	}
	s.notify(string(req.SessionId), SessionUpdate{
		SessionUpdate: "session_info_update",
		Title:         req.Title,
	})
	return map[string]any{}, nil
}

func (s *Server) permissionMode() string {
	if s.cfg != nil && s.cfg.Permission != nil {
		return s.cfg.Permission.Default
	}
	return "semi-ask"
}

func availableCommands() []AvailableCommand {
	skills := skill.List()
	out := make([]AvailableCommand, 0, len(skills))
	for _, sk := range skills {
		desc := sk.Description
		if desc == "" {
			desc = sk.WhenToUse
		}
		out = append(out, AvailableCommand{Name: sk.Name, Description: desc})
	}
	return out
}

func (s *Server) notifyCommands(sessionID string) {
	s.notify(sessionID, SessionUpdate{
		SessionUpdate:     "available_commands_update",
		AvailableCommands: availableCommands(),
	})
}

func (s *Server) modes(orch *agent.Agent) *SessionModeState {
	current := agent.ModeAuto
	if orch != nil {
		current = orch.CurrentMode()
	}
	return &SessionModeState{
		CurrentModeId: SessionModeId(string(current)),
		AvailableModes: []SessionMode{
			{ID: "build", Name: "Build", Description: "Write and modify code with full tool access"},
			{ID: "plan", Name: "Plan", Description: "Design and plan without implementation"},
			{ID: "auto", Name: "Auto", Description: "Automatically choose the best approach"},
		},
	}
}

func (s *Server) configOptions(orch *agent.Agent) []SessionConfigOption {
	current := agent.ModeAuto
	provider, model := "", ""
	if orch != nil {
		current = orch.CurrentMode()
		if orch.Provider != nil {
			provider = orch.Provider.Name()
		}
		model = orch.Model.ID
	}
	opts := []SessionConfigOption{
		{
			ID:           "mode",
			Name:         "Session Mode",
			Description:  "Controls how the agent requests permission",
			Category:     ConfigCategoryMode,
			Type:         "select",
			CurrentValue: string(current),
			Options: []SessionConfigSelectOption{
				{Value: "build", Name: "Build", Description: "Write and modify code with full tool access"},
				{Value: "plan", Name: "Plan", Description: "Design and plan without implementation"},
				{Value: "auto", Name: "Auto", Description: "Automatically choose the best approach"},
			},
		},
	}
	if models := s.modelOptions(provider); len(models) > 0 {
		if model == "" {
			model = string(models[0].Value)
		}
		opts = append(opts, SessionConfigOption{
			ID:           "model",
			Name:         "Model",
			Category:     ConfigCategoryModel,
			Type:         "select",
			CurrentValue: model,
			Options:      models,
		})
	}
	return opts
}

func (s *Server) modelOptions(provider string) []SessionConfigSelectOption {
	if s.llmMgr == nil {
		return nil
	}
	if !s.llmMgr.Ready() {
		s.llmMgr.WaitUntilReadyTimeout(2 * time.Second)
	}
	providers := []string{provider}
	if provider == "" {
		providers = s.providerNames()
	}
	seen := make(map[string]bool)
	out := make([]SessionConfigSelectOption, 0)
	for _, p := range providers {
		models, ok := s.llmMgr.ModelsNow(p)
		if !ok {
			continue
		}
		for _, m := range models {
			if m.ID == "" || seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			out = append(out, SessionConfigSelectOption{Value: SessionConfigValueId(m.ID), Name: m.ID})
		}
	}
	return out
}

func (s *Server) sessionModelState(orch *agent.Agent) *SessionModelState {
	current := ""
	if orch != nil {
		current = orch.Model.ID
	}
	opts := s.modelOptions("")
	if len(opts) == 0 {
		return nil
	}
	models := make([]ModelInfo, 0, len(opts))
	for _, o := range opts {
		models = append(models, ModelInfo{ModelId: string(o.Value), Name: o.Name})
	}
	if current == "" {
		current = string(opts[0].Value)
	}
	return &SessionModelState{
		CurrentModelId:  current,
		AvailableModels: models,
	}
}

func (s *Server) providerNames() []string {
	if s.cfg == nil {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(s.cfg.Providers))
	for _, p := range s.cfg.Providers {
		name := p.Provider
		if name == "" {
			name = p.Driver
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func (s *Server) notifyConfigOptions(acpSess *ACPSession) {
	if acpSess == nil {
		return
	}
	s.notify(acpSess.ID, SessionUpdate{
		SessionUpdate: "config_option_update",
		ConfigOptions: s.configOptions(acpSess.Agent),
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

func applySystemPrompt(orch *agent.Agent, sp *SystemPrompt) error {
	if sp == nil || orch == nil {
		return nil
	}
	if len(sp.Content) == 0 {
		return &RPCError{Code: ErrCodeInvalidParams, Message: "systemPrompt content must not be empty"}
	}
	var parts []string
	for _, b := range sp.Content {
		if b.Type != "text" {
			return &RPCError{Code: ErrCodeInvalidParams, Message: "systemPrompt only supports text content blocks"}
		}
		parts = append(parts, b.Text)
	}
	text := strings.Join(parts, "\n\n")
	switch sp.Mode {
	case SystemPromptAppend:
		if orch.SystemPrompt != "" {
			orch.SystemPrompt = orch.SystemPrompt + "\n\n" + text
		} else {
			orch.SystemPrompt = text
		}
	case SystemPromptOverride:
		orch.SystemPrompt = text
	default:
		return &RPCError{Code: ErrCodeInvalidParams, Message: "unsupported systemPrompt mode: " + string(sp.Mode)}
	}
	return nil
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

	if prov, mdl, err := s.resolveModel("", ""); err == nil {
		orch.SetModelProvider(mdl, prov)
	} else {
		s.log.Fdebug("acp: no model resolved for new session: %v", err)
	}
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
	case map[string]any:
		if text, _ := c["text"].(string); text != "" {
			return &ContentBlock{Type: "text", Text: text}
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
