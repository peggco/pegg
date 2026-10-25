package tui

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/tui/components"
)

func (a *App) ensureRuntimeMapsLocked() {
	if a.runtimes == nil {
		a.runtimes = make(map[string]*sessionRuntime)
	}
	if a.agentSessions == nil {
		a.agentSessions = make(map[string]string)
	}
	if a.subs == nil {
		a.subs = make(map[string]*agentTranscript)
	}
	if a.subItemByID == nil {
		a.subItemByID = make(map[string]*components.ChatItem)
	}
}

func (a *App) newAgentLocked() *agent.Agent {
	var (
		ag  *agent.Agent
		err error
	)
	switch {
	case len(a.runtimes) == 0 && a.deps.Agent != nil:
		ag = a.deps.Agent
	case a.deps.AgentFactory != nil:
		ag, err = a.deps.AgentFactory()
	case a.deps.Agent != nil:
		ag = a.deps.Agent.Clone("orchestrator")
	default:
		ag, err = agents.New("orchestrator")
	}
	if err != nil || ag == nil {
		return nil
	}
	if ag.Bus == nil {
		ag.Bus = a.bus
	}
	ag.SetMode(a.mode)
	if a.model.model.ID != "" && a.deps.LLM != nil {
		if prov, perr := a.deps.LLM.Provider(a.model.provider); perr == nil {
			ag.SetModelProvider(a.model.model, prov)
		}
	}
	ag.ReasoningEffort = a.reasoningEffort
	ag.Attachments = nil
	return ag
}

func (a *App) runtimeForLocked(sessionID string) *sessionRuntime {
	a.ensureRuntimeMapsLocked()
	if rt, ok := a.runtimes[sessionID]; ok && rt != nil && rt.agent != nil {
		a.agentSessions[rt.agent.ID] = sessionID
		return rt
	}
	ag := a.newAgentLocked()
	if ag == nil {
		return nil
	}
	rt := &sessionRuntime{
		id:              sessionID,
		agent:           ag,
		model:           a.model,
		reasoningEffort: a.reasoningEffort,
		mode:            a.mode,
	}
	a.runtimes[sessionID] = rt
	a.agentSessions[ag.ID] = sessionID
	return rt
}

func (a *App) currentSessionIDLocked() string {
	if a.session != nil {
		return a.session.info.ID
	}
	return ""
}

func (a *App) remapRuntimeLocked(oldID, newID, agentID string) {
	a.ensureRuntimeMapsLocked()
	if oldID == newID || newID == "" {
		return
	}
	rt := a.runtimes[oldID]
	if rt == nil {
		if agentID != "" {
			a.agentSessions[agentID] = newID
		}
		return
	}
	delete(a.runtimes, oldID)
	rt.id = newID
	a.runtimes[newID] = rt
	if rt.agent != nil {
		a.agentSessions[rt.agent.ID] = newID
	}
}

func (a *App) resetTranscriptsLocked() {
	a.main = nil
	a.subs = make(map[string]*agentTranscript)
	a.subItemByID = make(map[string]*components.ChatItem)
	a.loadedFloor = 0
	a.chat.Clear()
}

func (a *App) ownsAgentLocked(agentID, parentID string) bool {
	a.ensureRuntimeMapsLocked()
	if a.handlingAttached {
		return true
	}
	sid, ok := a.agentSessions[agentID]
	if !ok && parentID != "" {
		if psid, pok := a.agentSessions[parentID]; pok {
			sid, ok = psid, true
			a.agentSessions[agentID] = psid
		}
	}
	current := a.currentSessionIDLocked()
	if ok {
		return sid == current
	}
	if a.agent != nil && current == "" {
		return true
	}
	return false
}

func (a *App) acceptAgentEventLocked(agentID, parentID string) bool {
	if !a.ownsAgentLocked(agentID, parentID) {
		return false
	}
	if a.handlingAttached {
		return true
	}
	if a.attachCancel == nil || a.followSessionID == "" {
		return true
	}
	sid, ok := a.agentSessions[agentID]
	if !ok && parentID != "" {
		sid, ok = a.agentSessions[parentID]
	}
	return !ok || sid != a.followSessionID
}
