package acp

import (
	"context"

	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/session"
)

func (s *Server) startSessionFollow(acpSess *ACPSession) {
	if s.live == nil || acpSess == nil || acpSess.PeggSessionID == "" {
		return
	}
	snapshot, since, err := s.live.SnapshotSession(acpSess.PeggSessionID, false)
	if err != nil {
		return
	}
	sub := s.live.SubscribeSession(acpSess.PeggSessionID, false, since)
	acpSess.SetAttach(func() { sub.Close() }, "")
	go s.consumeFollow(acpSess, snapshot, sub)
}

func (s *Server) consumeFollow(acpSess *ACPSession, snapshot []live.Event, sub *live.Subscription) {
	defer sub.Close()
	st := &promptState{
		sessionID: acpSess.ID,
		textIDs:   make(map[string]string),
		thoughtID: make(map[string]string),
		oldFiles:  make(map[string]string),
	}
	for i := range snapshot {
		s.handleFollowEvent(acpSess, st, snapshot[i])
	}
	for e := range sub.C {
		s.handleFollowEvent(acpSess, st, e)
	}
}

func (s *Server) handleFollowEvent(acpSess *ACPSession, st *promptState, e live.Event) {
	switch e.Type {
	case live.EventRunStarted:
		st.textIDs = make(map[string]string)
		st.thoughtID = make(map[string]string)
		st.oldFiles = make(map[string]string)
		s.notify(acpSess.ID, SessionUpdate{
			SessionUpdate: "run_state_update",
			RunID:         e.Run.ID,
			RunState:      string(session.RunStateRunning),
			BaseSeq:       e.Run.BaseSeq,
		})
	case live.EventRunEvent:
		s.handleAttachedRunEvent(st, acpSess, &e.Run, e.Journal)
	case live.EventRunFinished:
		s.notify(acpSess.ID, SessionUpdate{
			SessionUpdate: "run_state_update",
			RunID:         e.Run.ID,
			RunState:      string(e.State),
			Error:         e.Error,
		})
	}
}

func (s *Server) handleAttachedRunEvent(st *promptState, acpSess *ACPSession, run *session.Run, e session.RunEvent) {
	if e.Type == session.RunEventAskResolved {
		s.resolveAsk("run:"+run.ID, run.ID, e.AgentID, acpSess.ID)
		return
	}
	ev, ok := live.DecodeAgentEvent(e)
	if !ok {
		return
	}
	switch ev.Type {
	case session.AgentEventInput:
		if ev.Input != "" {
			s.notify(st.sessionID, SessionUpdate{
				SessionUpdate: "user_message_chunk",
				MessageID:     uuid.NewString(),
				Content:       &ContentBlock{Type: "text", Text: ev.Input},
			})
		}
	case session.AgentEventStarted:
		if ev.ParentAgentID != "" {
			s.notify(st.sessionID, SessionUpdate{
				SessionUpdate: "subagent",
				AgentID:       ev.AgentID,
				AgentName:     ev.AgentName,
				Status:        "running",
			})
		}
	case session.AgentEventToken:
		if ev.Reasoning != "" {
			s.handleStreamEvent(st, acpSess, agent.StreamEvent{
				Type: agent.StreamReasoning, AgentID: ev.AgentID, AgentName: ev.AgentName, Reasoning: ev.Reasoning,
			})
		}
		if ev.Content != "" {
			s.handleStreamEvent(st, acpSess, agent.StreamEvent{
				Type: agent.StreamToken, AgentID: ev.AgentID, AgentName: ev.AgentName, Content: ev.Content,
			})
		}
	case session.AgentEventMessage:
		delete(st.textIDs, ev.AgentID)
		delete(st.thoughtID, ev.AgentID)
	case session.AgentEventToolCall:
		call := ev.ToolCall
		s.handleStreamEvent(st, acpSess, agent.StreamEvent{
			Type: agent.StreamToolCall, AgentID: ev.AgentID, AgentName: ev.AgentName, ToolCall: &call,
		})
	case session.AgentEventToolResult:
		call := ev.ToolCall
		stream := agent.StreamEvent{
			Type: agent.StreamToolResult, AgentID: ev.AgentID, AgentName: ev.AgentName,
			ToolCall: &call, ToolOutput: ev.ToolOutput,
		}
		if ev.ToolError != "" {
			stream.ToolErr = stringError(ev.ToolError)
		}
		s.handleStreamEvent(st, acpSess, stream)
	case session.AgentEventUsage:
		s.notify(st.sessionID, SessionUpdate{
			SessionUpdate: "usage_update",
			Used:          ev.Usage.TotalTokens,
			Size:          200000,
			Cost:          &Cost{Amount: ev.Usage.Cost, Currency: "USD"},
		})
	case session.AgentEventAsk:
		if s.runs == nil {
			return
		}
		runID := run.ID
		questions := ev.Questions
		ask := agent.AgentAsk{AgentID: ev.AgentID, AgentName: ev.AgentName, Questions: questions}
		askCtx, cancel := context.WithCancel(context.Background())
		key := "run:" + runID
		p := s.registerAsk(key, acpSess.ID, cancel)
		go func() {
			defer s.endAsk(key)
			answers, err := s.askClient(askCtx, acpSess, ask)
			if p.resolved.Load() {
				return
			}
			if err != nil {
				answers = deniedAnswers(questions)
			}
			if answers == nil {
				answers = map[string]string{}
			}
			_ = s.runs.Answer(runID, answers)
		}()
	case session.AgentEventError:
		if ev.ToolError != "" {
			s.notify(st.sessionID, SessionUpdate{SessionUpdate: "error", Error: ev.ToolError})
		}
	case session.AgentEventErrorMessage:
		if ev.ToolError != "" {
			s.notify(st.sessionID, SessionUpdate{SessionUpdate: "agent_warning", Error: ev.ToolError})
		}
	case session.AgentEventErrorMessageFinish:
		s.notify(st.sessionID, SessionUpdate{SessionUpdate: "agent_warning_clear"})
	case session.AgentEventCompaction:
		s.handleStreamEvent(st, acpSess, agent.StreamEvent{
			Type: agent.StreamCompaction, AgentID: ev.AgentID, AgentName: ev.AgentName,
			Strategy: ev.Strategy, Messages: ev.Messages, Tokens: ev.Tokens,
		})
	}
}

type stringError string

func (e stringError) Error() string { return string(e) }
