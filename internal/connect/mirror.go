package connect

import (
	"context"
	"fmt"
	"time"

	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func (c *Connector) startLiveMirror(ctx context.Context) {
	if c.deps.Live == nil {
		return
	}
	snapshot, since, err := c.deps.Live.SnapshotAll(true)
	if err != nil {
		c.log.Fwarn("connect: snapshot live runs: %v", err)
	}
	sub := c.deps.Live.SubscribeAll(true, since)

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
		if e.Local {
			return
		}
		c.emit(eventRunStarted, e.Run.SessionID, e.Run.ID, runStartedPayload{
			RunID:     e.Run.ID,
			SessionID: e.Run.SessionID,
			AgentID:   e.Run.AgentID,
			Provider:  e.Run.Provider,
			Model:     e.Run.Model,
			BaseSeq:   e.Run.BaseSeq,
		})
	case live.EventRunEvent:
		if e.Local && !c.runs.knows(e.Run.ID) {
			return
		}
		c.handleLiveEvent(e)
	case live.EventRunFinished:
		if e.Local {
			return
		}
		c.emitMirrorState(e.Run, e.State, e.Error)
	}
}

func (c *Connector) enrichRun(info *RunInfo) {
	if c.deps.Live == nil || info == nil || info.ID == "" {
		return
	}
	sum, err := c.deps.Live.SummarizeRun(info.ID, info.AgentID)
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

func (c *Connector) handleLiveEvent(e live.Event) {
	ev, ok := live.DecodeAgentEvent(e.Journal)
	if !ok {
		return
	}
	isSubagent := e.Journal.ParentAgentID != "" ||
		(e.Run.AgentID != "" && e.Journal.AgentID != "" && e.Journal.AgentID != e.Run.AgentID)
	if !isSubagent {
		if e.Local {
			return
		}
		c.handleMainLiveEvent(e.Run, ev)
		return
	}
	c.handleSubagentLiveEvent(e.Run, e.Journal, ev)
}

func (c *Connector) handleMainLiveEvent(r session.Run, ev live.AgentEvent) {
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
	case session.AgentEventFinished:
		c.emit(eventRunDone, r.SessionID, r.ID, runDonePayload{
			Output: ev.Output, Usage: ev.Usage, Iterations: ev.Iterations, FinishReason: ev.FinishReason,
		})
	case session.AgentEventError:
		c.emit(eventRunError, r.SessionID, r.ID, runErrorPayload{Message: ev.ToolError})
	case session.AgentEventErrorMessage:
		c.emit(eventRunWarning, r.SessionID, r.ID, runWarningPayload{Message: ev.ToolError})
	case session.AgentEventErrorMessageFinish:
		c.emit(eventRunWarnClear, r.SessionID, r.ID, nil)
	case session.AgentEventCompaction:
		c.emit(eventRunCompaction, r.SessionID, r.ID, runCompactionPayload{
			Strategy: ev.Strategy, Messages: ev.Messages, Tokens: ev.Tokens,
		})
	}
}

func (c *Connector) handleSubagentLiveEvent(r session.Run, e session.RunEvent, ev live.AgentEvent) {
	agentID := ev.AgentID
	if agentID == "" {
		agentID = e.AgentID
	}
	switch ev.Type {
	case session.AgentEventStarted:
		c.emit(eventRunSubagentStarted, r.SessionID, r.ID, runSubagentStartedPayload{
			AgentID:       agentID,
			AgentName:     ev.AgentName,
			DisplayName:   ev.DisplayName,
			ParentAgentID: ev.ParentAgentID,
		})
	case session.AgentEventInput:
		msg := llm.UserMessage(ev.Input)
		if len(ev.Attachments) > 0 {
			msg = llm.UserMessage(llm.ContentWithAttachments(ev.Input, ev.Attachments))
		}
		c.emitSubagentMessage(r, e, agentID, msg)
	case session.AgentEventMessage:
		c.emitSubagentMessage(r, e, agentID, ev.Message)
	case session.AgentEventToolResult:
		content := ev.ToolOutput
		if content == "" {
			content = ev.ToolError
		}
		c.emitSubagentMessage(r, e, agentID, llm.ToolMessage(content, ev.CallID))
	case session.AgentEventUsage:
		c.emit(eventRunSubagentUsage, r.SessionID, r.ID, runSubagentUsagePayload{
			AgentID: agentID,
			Usage:   ev.Usage,
		})
	case session.AgentEventFinished:
		c.emit(eventRunSubagentDone, r.SessionID, r.ID, runSubagentDonePayload{
			AgentID: agentID,
			Output:  ev.Output,
			Usage:   ev.Usage,
		})
	case session.AgentEventError:
		message := ev.ToolError
		if message == "" {
			message = "subagent failed"
		}
		c.emit(eventRunSubagentError, r.SessionID, r.ID, runSubagentErrorPayload{
			AgentID: agentID,
			Message: message,
		})
	}
}

func (c *Connector) emitSubagentMessage(r session.Run, e session.RunEvent, agentID string, msg llm.Message) {
	created := e.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	c.emit(eventRunSubagentMessage, r.SessionID, r.ID, runSubagentMessagePayload{
		AgentID: agentID,
		Message: SessionMessage{
			ID:         fmt.Sprintf("subagent-%s-%d", agentID, e.Seq),
			SessionID:  e.SessionID,
			Seq:        int(e.Seq),
			Role:       string(msg.Role),
			Content:    msg.Content,
			Reasoning:  msg.Reasoning,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
			CreatedAt:  created.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
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
