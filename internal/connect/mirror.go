package connect

import (
	"context"

	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/session"
)

func (c *Connector) startLiveMirror(ctx context.Context) {
	if c.deps.Live == nil {
		return
	}
	snapshot, since, err := c.deps.Live.SnapshotAll(false)
	if err != nil {
		c.log.Fwarn("connect: snapshot live runs: %v", err)
	}
	sub := c.deps.Live.SubscribeAll(false, since)

	c.mirrorMu.Lock()
	if c.liveSub != nil {
		c.liveSub.Close()
	}
	c.liveSub = sub
	c.mirrorMu.Unlock()

	go func() {
		defer sub.Close()
		for i := range snapshot {
			c.forwardLive(snapshot[i])
		}
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-sub.C:
				if !ok {
					return
				}
				c.forwardLive(e)
			}
		}
	}()
}

func (c *Connector) stopLiveMirror() {
	c.mirrorMu.Lock()
	sub := c.liveSub
	c.liveSub = nil
	c.mirrorMu.Unlock()
	if sub != nil {
		sub.Close()
	}
}

func (c *Connector) forwardLive(e live.Event) {
	switch e.Type {
	case live.EventRunStarted:
		c.emit(eventRunStarted, e.Run.SessionID, e.Run.ID, runStartedPayload{
			RunID:     e.Run.ID,
			SessionID: e.Run.SessionID,
			AgentID:   e.Run.AgentID,
			Provider:  e.Run.Provider,
			Model:     e.Run.Model,
			BaseSeq:   e.Run.BaseSeq,
		})
	case live.EventRunEvent:
		c.handleMirrorEvent(e.Run, e.Journal)
	case live.EventRunFinished:
		c.emitMirrorState(e.Run, e.State, e.Error)
	}
}

func (c *Connector) enrichRun(info *RunInfo) {
	if c.deps.Live == nil || info == nil || info.ID == "" {
		return
	}
	sum, err := c.deps.Live.SummarizeRun(info.ID)
	if err != nil {
		return
	}
	info.Content = sum.Content
	info.Reasoning = sum.Reasoning
	info.EventSeq = sum.EventSeq
	info.Questions = sum.Questions
	info.Tools = make([]RunToolInfo, len(sum.Tools))
	for i, t := range sum.Tools {
		info.Tools[i] = RunToolInfo{
			CallID: t.CallID, ToolName: t.ToolName, Args: t.Args,
			Output: t.Output, Error: t.Error, Done: t.Done,
		}
	}
}

func (c *Connector) handleMirrorEvent(r session.Run, e session.RunEvent) bool {
	if e.AgentID != "" && r.AgentID != "" && e.AgentID != r.AgentID {
		switch e.Type {
		case session.AgentEventAsk, session.RunEventAskResolved:
		default:
			return false
		}
	}
	if e.Type == session.RunEventState {
		if state, errMsg, ok := live.DecodeRunState(e); ok {
			c.emitMirrorState(r, state, errMsg)
			return true
		}
		return false
	}
	ev, ok := live.DecodeAgentEvent(e)
	if !ok {
		return false
	}
	switch ev.Type {
	case session.AgentEventInput:
		c.emit(eventRunInput, r.SessionID, r.ID, map[string]any{
			"input":       ev.Input,
			"attachments": ev.Attachments,
		})
	case session.AgentEventToken:
		if ev.Content != "" || ev.Reasoning != "" {
			c.emit(eventRunToken, r.SessionID, r.ID, runTokenPayload{Content: ev.Content, Reasoning: ev.Reasoning})
		}
	case session.AgentEventToolCall:
		c.emit(eventRunToolCall, r.SessionID, r.ID, runToolCallPayload{
			CallID: ev.CallID, ToolName: ev.ToolName, Args: ev.ToolCall.Function.Arguments,
		})
	case session.AgentEventToolResult:
		c.emit(eventRunToolResult, r.SessionID, r.ID, runToolResultPayload{
			CallID: ev.CallID, ToolName: ev.ToolName, Output: ev.ToolOutput, Error: ev.ToolError,
		})
	case session.AgentEventUsage:
		c.emit(eventRunUsage, r.SessionID, r.ID, runUsagePayload{Usage: ev.Usage})
	case session.AgentEventAsk:
		name := eventRunAsk
		if len(ev.Questions) == 1 && ev.Questions[0].ID == "decision" {
			name = eventRunPermission
		}
		c.emit(name, r.SessionID, r.ID, runAskPayload{AgentName: ev.AgentName, Questions: ev.Questions})
	case session.RunEventAskResolved:
		c.emit(eventRunAskResolved, r.SessionID, r.ID, map[string]any{
			"run_id":  r.ID,
			"answers": ev.Answers,
		})
	case session.AgentEventFinished:
		c.emit(eventRunDone, r.SessionID, r.ID, runDonePayload{
			Output: ev.Output, Usage: ev.Usage, Iterations: ev.Iterations, FinishReason: ev.FinishReason,
		})
		return true
	case session.AgentEventError:
		c.emit(eventRunError, r.SessionID, r.ID, runErrorPayload{Message: ev.ToolError})
		return true
	case session.AgentEventErrorMessage:
		c.emit(eventRunWarning, r.SessionID, r.ID, runWarningPayload{Message: ev.ToolError})
	case session.AgentEventErrorMessageFinish:
		c.emit(eventRunWarnClear, r.SessionID, r.ID, nil)
	case session.AgentEventCompaction:
		c.emit(eventRunCompaction, r.SessionID, r.ID, runCompactionPayload{
			Strategy: ev.Strategy, Messages: ev.Messages, Tokens: ev.Tokens,
		})
	}
	return false
}

func (c *Connector) emitMirrorState(r session.Run, state session.RunState, errMsg string) {
	switch state {
	case session.RunStateDone:
		c.emit(eventRunDone, r.SessionID, r.ID, runDonePayload{})
	case session.RunStateError:
		if errMsg == "" {
			errMsg = "run failed"
		}
		c.emit(eventRunError, r.SessionID, r.ID, runErrorPayload{Message: errMsg})
	case session.RunStateCanceled:
		c.emit(eventRunError, r.SessionID, r.ID, runErrorPayload{Message: "run canceled"})
	}
}
