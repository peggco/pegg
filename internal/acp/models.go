package acp

import (
	"context"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/llm"
)

type modelListParams struct {
	Provider string `json:"provider,omitempty"`
}

type modelListResult struct {
	Models []modelEntry `json:"models"`
}

type modelEntry struct {
	Provider string    `json:"provider"`
	Model    llm.Model `json:"model"`
}

type sessionSetModelParams struct {
	SessionId       SessionId `json:"sessionId"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	ReasoningEffort string    `json:"reasoningEffort,omitempty"`
}

func (s *Server) handleListModels(ctx context.Context, params json.RawMessage) (any, error) {
	var req modelListParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	if s.llmMgr == nil {
		return nil, ErrModelUnavailable
	}
	if s.cfg == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "config unavailable"}
	}
	s.llmMgr.WaitUntilReady()

	providers := make([]string, 0, len(s.cfg.Providers))
	seen := make(map[string]bool, len(s.cfg.Providers))
	for _, p := range s.cfg.Providers {
		name := p.Provider
		if name == "" {
			name = p.Driver
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		providers = append(providers, name)
	}

	out := make([]modelEntry, 0)
	for _, name := range providers {
		if req.Provider != "" && req.Provider != name {
			continue
		}
		models, err := s.llmMgr.Models(name)
		if err != nil {
			continue
		}
		for _, mdl := range models {
			out = append(out, modelEntry{Provider: name, Model: mdl})
		}
	}
	return modelListResult{Models: out}, nil
}

func (s *Server) handleSessionSetModel(ctx context.Context, params json.RawMessage) (any, error) {
	var req sessionSetModelParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/set_model params"}
	}
	if req.Provider == "" || req.Model == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "provider and model are required"}
	}
	acpSess, ok := s.getSession(string(req.SessionId))
	if !ok {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	if acpSess.IsRunning() {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "cannot change model while a prompt is running"}
	}
	prov, mdl, err := s.resolveModel(req.Provider, req.Model)
	if err != nil {
		return nil, err
	}
	if acpSess.Agent != nil {
		acpSess.Agent.SetModelProvider(mdl, prov)
		if req.ReasoningEffort != "" {
			acpSess.Agent.ReasoningEffort = req.ReasoningEffort
		}
	}
	acpSess.Provider = prov.Name()
	acpSess.Model = mdl.ID
	if acpSess.PeggSessionID != "" {
		_ = s.sessions.SetModel(acpSess.PeggSessionID, acpSess.Provider, acpSess.Model)
	}
	s.notify(acpSess.ID, SessionUpdate{
		SessionUpdate: "current_model_update",
		Provider:      acpSess.Provider,
		Model:         acpSess.Model,
	})
	return map[string]string{"provider": acpSess.Provider, "model": acpSess.Model}, nil
}
