package sdk

import (
	"context"
	"encoding/json"

	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/session"
)

type RunInfo struct {
	ID        string
	SessionID string
	State     string
	AgentID   string
	AgentName string
	Provider  string
	Model     string
	BaseSeq   int
	Owner     string
}

type RunEvent struct {
	Seq       int64
	RunID     string
	SessionID string
	Type      string
	AgentID   string
	AgentName string
	Data      json.RawMessage
}

func (e *Engine) ActiveRun(sessionID string) (RunInfo, error) {
	if err := e.checkOpen(); err != nil {
		return RunInfo{}, err
	}
	if e.runs == nil {
		return RunInfo{}, session.ErrRunUnsupported
	}
	run, err := e.runs.Active(sessionID)
	if err != nil {
		return RunInfo{}, err
	}
	return runInfo(run), nil
}

func (e *Engine) ListActiveRuns() ([]RunInfo, error) {
	if err := e.checkOpen(); err != nil {
		return nil, err
	}
	if e.runs == nil {
		return nil, session.ErrRunUnsupported
	}
	runs, err := e.runs.ActiveRuns()
	if err != nil {
		return nil, err
	}
	out := make([]RunInfo, len(runs))
	for i := range runs {
		out[i] = runInfo(&runs[i])
	}
	return out, nil
}

func (e *Engine) AttachRun(ctx context.Context, sessionID string, handler func(RunEvent) error) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if e.runs == nil {
		return session.ErrRunUnsupported
	}
	run, err := e.runs.Active(sessionID)
	if err != nil {
		return err
	}
	if e.live == nil {
		return e.runs.Watch(ctx, run.ID, 0, func(ev session.RunEvent) error {
			return deliverRunEvent(handler, ev)
		})
	}
	snapshot, since, err := e.live.SnapshotSession(sessionID, true)
	if err != nil {
		return err
	}
	sub := e.live.SubscribeSession(sessionID, true, since)
	defer sub.Close()
	for i := range snapshot {
		if snapshot[i].Type != live.EventRunEvent {
			continue
		}
		if err := deliverRunEvent(handler, snapshot[i].Journal); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-sub.C:
			if !ok {
				return nil
			}
			if ev.Type == live.EventRunEvent {
				if err := deliverRunEvent(handler, ev.Journal); err != nil {
					return err
				}
			}
			if ev.Type == live.EventRunFinished {
				return nil
			}
		}
	}
}

func (e *Engine) FollowSession(ctx context.Context, sessionID string, handler func(RunEvent) error) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if e.live == nil {
		return session.ErrRunUnsupported
	}
	snapshot, since, err := e.live.SnapshotSession(sessionID, true)
	if err != nil {
		return err
	}
	sub := e.live.SubscribeSession(sessionID, true, since)
	defer sub.Close()
	for i := range snapshot {
		if snapshot[i].Type == live.EventRunEvent {
			if err := deliverRunEvent(handler, snapshot[i].Journal); err != nil {
				return err
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-sub.C:
			if !ok {
				return nil
			}
			if ev.Type == live.EventRunEvent {
				if err := deliverRunEvent(handler, ev.Journal); err != nil {
					return err
				}
			}
		}
	}
}

func deliverRunEvent(handler func(RunEvent) error, ev session.RunEvent) error {
	if handler == nil {
		return nil
	}
	return handler(RunEvent{
		Seq:       ev.Seq,
		RunID:     ev.RunID,
		SessionID: ev.SessionID,
		Type:      ev.Type,
		AgentID:   ev.AgentID,
		AgentName: ev.AgentName,
		Data:      ev.Data,
	})
}

func (e *Engine) CancelRun(runID string) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if e.runs == nil {
		return session.ErrRunUnsupported
	}
	return e.runs.Cancel(runID)
}

func runInfo(r *session.Run) RunInfo {
	if r == nil {
		return RunInfo{}
	}
	return RunInfo{
		ID:        r.ID,
		SessionID: r.SessionID,
		State:     string(r.State),
		AgentID:   r.AgentID,
		AgentName: r.AgentName,
		Provider:  r.Provider,
		Model:     r.Model,
		BaseSeq:   r.BaseSeq,
		Owner:     r.OwnerInstance,
	}
}
