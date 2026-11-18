package acp

import (
	"context"
	"sort"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/changejournal"
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
	Removed        int                        `json:"removed"`
	SnapshotId     string                     `json:"snapshotId,omitempty"`
	Files          []session.RevertFileChange `json:"files,omitempty"`
	TotalAdditions int                        `json:"totalAdditions"`
	TotalDeletions int                        `json:"totalDeletions"`
}

type SessionUndoRevertParams struct {
	SessionId  SessionId `json:"sessionId"`
	SnapshotId string    `json:"snapshotId"`
}

type SessionUndoRevertResult struct {
	Restored   bool   `json:"restored"`
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
	summary := session.SummarizeRevert(removed)
	summary = s.applyJournalSummary(acpSess.PeggSessionID, snapshotID, summary)
	s.log.Finfo("acp: reverted session %s to message %s (%d removed)", acpSess.PeggSessionID, target.ID, len(removed))
	return SessionRevertResult{
		Removed:        len(removed),
		SnapshotId:     snapshotID,
		Files:          summary.Files,
		TotalAdditions: summary.TotalAdditions,
		TotalDeletions: summary.TotalDeletions,
	}, nil
}

func (s *Server) applyJournalSummary(sessionID, revertID string, summary session.RevertSummary) session.RevertSummary {
	s.journalMu.Lock()
	result, ok := s.journalResults[sessionID]
	s.journalMu.Unlock()
	if !ok || (result.RevertID != "" && result.RevertID != revertID) {
		return summary
	}
	byPath := make(map[string]int, len(summary.Files))
	for i, file := range summary.Files {
		byPath[file.Path] = i
	}
	for _, change := range result.Files {
		status := journalRevertStatus(change.Status)
		if index, exists := byPath[change.Path]; exists {
			summary.Files[index].Status = status
			summary.Files[index].Reason = change.Reason
			continue
		}
		byPath[change.Path] = len(summary.Files)
		summary.Files = append(summary.Files, session.RevertFileChange{
			Path:   change.Path,
			Status: status,
			Reason: change.Reason,
		})
	}
	sort.Slice(summary.Files, func(i, j int) bool { return summary.Files[i].Path < summary.Files[j].Path })
	return summary
}

func journalRevertStatus(status changejournal.Status) string {
	switch status {
	case changejournal.StatusRestored, changejournal.StatusDeleted, changejournal.StatusAlreadyReverted:
		return session.RevertStatusReverted
	case changejournal.StatusConflict, changejournal.StatusMissingBackup:
		return session.RevertStatusConflict
	default:
		return session.RevertStatusSkipped
	}
}

func (s *Server) handleSessionUndoRevert(_ context.Context, params json.RawMessage) (any, error) {
	var req SessionUndoRevertParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid params"}
	}
	if req.SessionId == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "sessionId is required"}
	}
	if req.SnapshotId == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "snapshotId is required"}
	}
	acpSess, ok := s.getSession(string(req.SessionId))
	if !ok {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	if acpSess.PeggSessionID == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session has no stored history"}
	}
	if acpSess.IsRunning() {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "a prompt is running for this session"}
	}
	if err := s.sessions.UndoRevert(acpSess.PeggSessionID, req.SnapshotId); err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "undo revert: " + err.Error()}
	}
	s.log.Finfo("acp: restored session %s from snapshot %s", acpSess.PeggSessionID, req.SnapshotId)
	return SessionUndoRevertResult{Restored: true, SnapshotId: req.SnapshotId}, nil
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
