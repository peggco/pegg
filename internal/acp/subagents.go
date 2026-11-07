package acp

import (
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
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "running",
		Title:         attach.StripBlock(ev.Input),
		Meta:          Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName},
	})
}

func (s *Server) handleSubagentFinished(ev agent.AgentFinished) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "finished",
		RawOutput:     truncate(ev.Output, 64*1024),
		Meta:          Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName},
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
	s.notifyPegg(acpSess.ID, SessionUpdate{
		SessionUpdate: "subagent",
		Status:        "error",
		Meta:          Meta{"com.pegg/agentId": ev.AgentID, "com.pegg/agentName": ev.AgentName, "com.pegg/error": message},
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
