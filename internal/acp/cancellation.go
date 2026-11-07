package acp

import (
	"context"

	json "github.com/goccy/go-json"
)

type SessionCancelParams struct {
	Meta      Meta      `json:"_meta,omitempty"`
	SessionId SessionId `json:"sessionId"`
}

type CancelRequestParams struct {
	Meta      Meta `json:"_meta,omitempty"`
	RequestId any  `json:"requestId"`
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
	var req CancelRequestParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil
	}
	if req.RequestId == nil {
		return nil
	}
	s.mu.RLock()
	var target *ACPSession
	for _, sess := range s.active {
		if sess.matchesRequest(req.RequestId) {
			target = sess
			break
		}
	}
	s.mu.RUnlock()
	if target == nil {
		return nil
	}
	target.Cancel()
	if s.runs != nil && target.PeggSessionID != "" {
		_ = s.runs.CancelSession(target.PeggSessionID)
	}
	return nil
}
