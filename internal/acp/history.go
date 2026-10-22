package acp

import (
	"context"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

type SessionMessageRef struct {
	SessionId        SessionId `json:"sessionId"`
	MessageId        string    `json:"messageId,omitempty"`
	UserMessageIndex *int      `json:"userMessageIndex,omitempty"`
}

type SessionForkResult struct {
	SessionId string `json:"sessionId"`
	Title     string `json:"title,omitempty"`
	ParentId  string `json:"parentId,omitempty"`
}

type SessionRevertResult struct {
	Removed    int    `json:"removed"`
	SnapshotId string `json:"snapshotId,omitempty"`
}

func (s *Server) handleSessionFork(_ context.Context, params json.RawMessage) (any, error) {
	ref, acpSess, err := s.resolveMessageRef(params)
	if err != nil {
		return nil, err
	}
	if acpSess.IsRunning() {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "a prompt is running for this session"}
	}
	target, err := s.messageTarget(acpSess.PeggSessionID, ref)
	if err != nil {
		return nil, err
	}
	fork, err := s.sessions.ForkBefore(acpSess.PeggSessionID, target.ID)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "fork session: " + err.Error()}
	}
	s.log.Finfo("acp: forked session %s before message %s into %s", acpSess.PeggSessionID, target.ID, fork.ID)
	return SessionForkResult{
		SessionId: fork.ID,
		Title:     fork.Title,
		ParentId:  acpSess.PeggSessionID,
	}, nil
}

func (s *Server) handleSessionRevert(_ context.Context, params json.RawMessage) (any, error) {
	ref, acpSess, err := s.resolveMessageRef(params)
	if err != nil {
		return nil, err
	}
	if acpSess.IsRunning() {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "a prompt is running for this session"}
	}
	target, err := s.messageTarget(acpSess.PeggSessionID, ref)
	if err != nil {
		return nil, err
	}
	snapshotID, removed, err := s.sessions.RevertFrom(acpSess.PeggSessionID, target.ID)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "revert session: " + err.Error()}
	}
	s.log.Finfo("acp: reverted session %s to message %s (%d removed)", acpSess.PeggSessionID, target.ID, len(removed))
	return SessionRevertResult{Removed: len(removed), SnapshotId: snapshotID}, nil
}

func (s *Server) resolveMessageRef(params json.RawMessage) (SessionMessageRef, *ACPSession, error) {
	var ref SessionMessageRef
	if err := json.Unmarshal(params, &ref); err != nil {
		return ref, nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid params"}
	}
	if ref.SessionId == "" {
		return ref, nil, &RPCError{Code: ErrCodeInvalidParams, Message: "sessionId is required"}
	}
	acpSess, ok := s.getSession(string(ref.SessionId))
	if !ok {
		return ref, nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	if acpSess.PeggSessionID == "" {
		return ref, nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session has no stored history"}
	}
	return ref, acpSess, nil
}

func (s *Server) messageTarget(peggSessionID string, ref SessionMessageRef) (session.Message, error) {
	msgs, err := s.sessions.Messages(peggSessionID)
	if err != nil {
		return session.Message{}, &RPCError{Code: ErrCodeInternal, Message: "read session messages: " + err.Error()}
	}
	if ref.MessageId != "" {
		for _, msg := range msgs {
			if msg.ID == ref.MessageId {
				return msg, nil
			}
		}
	}
	if ref.UserMessageIndex != nil {
		index := *ref.UserMessageIndex
		seen := 0
		for _, msg := range msgs {
			if msg.Role != llm.RoleUser {
				continue
			}
			if seen == index {
				return msg, nil
			}
			seen++
		}
	}
	return session.Message{}, &RPCError{Code: ErrCodeInvalidParams, Message: "message not found"}
}
