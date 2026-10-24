package acp

import (
	"context"
	json "github.com/goccy/go-json"
)

type SessionCancelParams struct {
	SessionId SessionId `json:"sessionId"`
}

type CancelRequestParams struct {
	Id any `json:"id"`
}

func (s *Server) handleSessionCancel(ctx context.Context, params json.RawMessage) error {
	var req SessionCancelParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil
	}
	if acpSess, ok := s.getSession(string(req.SessionId)); ok {
		acpSess.Cancel()
		if s.runs != nil && acpSess.PeggSessionID != "" {
			_ = s.runs.CancelSession(acpSess.PeggSessionID)
		}
	}
	return nil
}

func (s *Server) handleCancelRequest(ctx context.Context, params json.RawMessage) error {
	return nil
}
