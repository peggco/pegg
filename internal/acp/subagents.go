package acp

import (
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/attach"
)

func (s *Server) subscribeSubagents() {
	if s.bus == nil {
		return
	}
	_ = s.bus.Subscribe(agent.TopicAgentInput, s.handleSubagentInput)
	_ = s.bus.Subscribe(agent.TopicAgentFinished, s.handleSubagentFinished)
	_ = s.bus.Subscribe(agent.TopicAgentError, s.handleSubagentError)
	s.subscribeSubagentStream()
}

func (s *Server) handleSubagentInput(ev agent.AgentInput) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	s.mu.Lock()
	taskName := s.taskNames[ev.AgentID]
	if _, started := s.subagentStart[ev.AgentID]; !started {
		s.subagentStart[ev.AgentID] = time.Now()
	}
	s.mu.Unlock()
	meta := Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName}
	if taskName != "" {
		meta["com.pegg/taskName"] = taskName
	}
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "running",
		Title:         attach.StripBlock(ev.Input),
		Meta:          meta,
	})
}

func (s *Server) handleSubagentFinished(ev agent.AgentFinished) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	s.mu.Lock()
	taskName := s.taskNames[ev.AgentID]
	started := s.subagentStart[ev.AgentID]
	delete(s.taskNames, ev.AgentID)
	delete(s.subagentStart, ev.AgentID)
	s.mu.Unlock()
	meta := Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName}
	if taskName != "" {
		meta["com.pegg/taskName"] = taskName
	}
	if ev.Usage.TotalTokens > 0 || ev.Usage.Cost > 0 {
		meta["com.pegg/usage"] = map[string]any{"used": ev.Usage.TotalTokens, "cost": ev.Usage.Cost}
	}
	if !started.IsZero() {
		meta["com.pegg/durationMs"] = time.Since(started).Milliseconds()
	}
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "finished",
		RawOutput:     truncate(ev.Output, 64*1024),
		Meta:          meta,
	})
}

func (s *Server) handleSubagentError(ev agent.AgentError) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	message := "subagent failed"
	if ev.Err != nil {
		message = ev.Err.Error()
	}
	s.mu.Lock()
	taskName := s.taskNames[ev.AgentID]
	started := s.subagentStart[ev.AgentID]
	delete(s.taskNames, ev.AgentID)
	delete(s.subagentStart, ev.AgentID)
	s.mu.Unlock()
	meta := Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName, "com.pegg/error": message}
	if taskName != "" {
		meta["com.pegg/taskName"] = taskName
	}
	if !started.IsZero() {
		meta["com.pegg/durationMs"] = time.Since(started).Milliseconds()
	}
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "error",
		Meta:          meta,
	})
}

func (s *Server) subagentSession(agentID string) (*ACPSession, bool) {
	if agentID == "" {
		return nil, false
	}
	acpSess, ok := s.sessionForAgent(agentID)
	if !ok || acpSess.Agent == nil || acpSess.Agent.ID == agentID {
		return nil, false
	}
	return acpSess, true
}
