package tui

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

func messagesUpTo(msgs []session.Message, maxSeq int) []session.Message {
	if maxSeq <= 0 {
		return msgs
	}
	out := make([]session.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Seq <= maxSeq {
			out = append(out, m)
		}
	}
	return out
}

func (a *App) beginSessionFollow(info settings.SessionInfo) {
	a.stopAttach()
	var (
		snapshot []live.Event
		since    int64
		active   *session.Run
	)
	if a.deps.Live != nil {
		events, seq, err := a.deps.Live.SnapshotSession(info.ID, false)
		if err == nil {
			snapshot = events
			since = seq
			for i := range events {
				if events[i].Type == live.EventRunStarted && events[i].Run.State == session.RunStateRunning {
					r := events[i].Run
					active = &r
				}
			}
		}
	}
	if active != nil && active.BaseSeq > 0 {
		info.Messages = messagesUpTo(info.Messages, active.BaseSeq)
	}

	a.chatMu.Lock()
	a.session = &activeSession{info: info, viewID: info.ID}
	a.mode = agent.NormalizeMode(agent.Mode(info.Mode))
	if a.agent != nil {
		a.agent.SetMode(a.mode)
	}
	a.reasoningEffort = info.ReasoningEffort
	if a.deps.LLM != nil && info.Provider != "" && info.Model != "" {
		if res := a.deps.LLM.Select(llm.SelectRequest{
			Mode:     llm.SelectModeExact,
			Provider: info.Provider,
			Model:    info.Model,
		}); res.Err == nil {
			a.model = selectedModel{provider: res.Provider, model: res.Model}
		}
	}
	a.loadSessionIntoChatLocked()
	a.refreshHomeLocked()
	a.chatMu.Unlock()

	if a.deps.Live != nil {
		sub := a.deps.Live.SubscribeSession(info.ID, false, since)
		a.chatMu.Lock()
		a.attachCancel = sub.Close
		a.followSessionID = info.ID
		a.followGen++
		gen := a.followGen
		a.chatMu.Unlock()
		go a.consumeFollow(info.ID, gen, snapshot, sub)
	}
	a.requestRedraw()
}

func (a *App) ensureFollow(sessionID string) {
	if a.deps.Live == nil || sessionID == "" {
		return
	}
	a.chatMu.Lock()
	if a.followSessionID == sessionID && a.attachCancel != nil {
		a.chatMu.Unlock()
		return
	}
	a.chatMu.Unlock()

	snapshot, since, err := a.deps.Live.SnapshotSession(sessionID, false)
	if err != nil {
		return
	}
	sub := a.deps.Live.SubscribeSession(sessionID, false, since)

	a.chatMu.Lock()
	old := a.attachCancel
	a.attachCancel = sub.Close
	a.followSessionID = sessionID
	a.followGen++
	gen := a.followGen
	a.chatMu.Unlock()
	if old != nil {
		old()
	}
	go a.consumeFollow(sessionID, gen, snapshot, sub)
}

func (a *App) consumeFollow(sessionID string, gen int, snapshot []live.Event, sub *live.Subscription) {
	defer sub.Close()
	for i := range snapshot {
		if a.ctx.Err() != nil {
			return
		}
		a.handleFollowEvent(sessionID, gen, snapshot[i])
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			a.handleFollowEvent(sessionID, gen, e)
		}
	}
}

func (a *App) handleFollowEvent(sessionID string, gen int, e live.Event) {
	a.chatMu.Lock()
	current := a.followSessionID == sessionID && a.followGen == gen
	a.chatMu.Unlock()
	if !current {
		return
	}
	switch e.Type {
	case live.EventRunStarted:
		a.beginAttachedRun(e.Run)
	case live.EventRunEvent:
		a.handleAttachedEvent(e.Journal, e.Run.AgentID)
	case live.EventRunFinished:
		a.finishAttach(e.Run.SessionID, e.Run.ID)
	}
}

func (a *App) beginAttachedRun(r session.Run) {
	a.chatMu.Lock()
	if a.main != nil {
		a.main.assistant = nil
		a.main.thinking = nil
		a.main.done = false
		a.main.err = ""
	}
	a.attachRunID = r.ID
	a.attachAgentID = r.AgentID
	a.attachOwner = r.OwnerInstance
	a.running = true
	a.refreshHomeLocked()
	a.chatMu.Unlock()
	a.requestRedraw()
}

func (a *App) stopAttach() {
	a.chatMu.Lock()
	cancel := a.attachCancel
	wasAttached := a.attachRunID != ""
	a.attachCancel = nil
	a.attachRunID = ""
	a.attachAgentID = ""
	a.attachOwner = ""
	a.followSessionID = ""
	a.followGen++
	if wasAttached {
		a.running = false
	}
	a.refreshHomeLocked()
	a.chatMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if wasAttached {
		a.requestRedraw()
	}
}

func (a *App) finishAttach(sessionID, runID string) {
	if a.deps.Sessions == nil || sessionID == "" {
		return
	}
	sess, err := a.deps.Sessions.LatestInChain(sessionID)
	if err != nil {
		sess, err = a.deps.Sessions.Get(sessionID)
		if err != nil {
			return
		}
	}
	msgs, err := a.deps.Sessions.Messages(sess.ID)
	if err != nil {
		return
	}

	newRunActive := false
	if a.deps.Runs != nil {
		for _, id := range []string{sessionID, sess.ID} {
			if id == "" {
				continue
			}
			if active, aerr := a.deps.Runs.Active(id); aerr == nil && active != nil &&
				active.State == session.RunStateRunning && active.ID != runID {
				newRunActive = true
				break
			}
		}
	}

	a.chatMu.Lock()
	if a.attachRunID == runID {
		a.attachRunID = ""
		a.attachAgentID = ""
		a.attachOwner = ""
	}
	if a.session != nil && a.session.info.ID == sessionID {
		a.session.info.ID = sess.ID
		a.session.info.CompactionParentID = sess.CompactionParentID
		a.session.viewID = sess.ID
		a.session.info.Title = sess.Title
		a.session.info.Provider = sess.Provider
		a.session.info.Model = sess.Model
		a.session.info.Mode = sess.Mode
		a.session.info.ReasoningEffort = sess.ReasoningEffort
		a.session.info.Usage = sess.Usage
		a.session.info.Messages = msgs
		if newRunActive {
			a.seedHistoryFromSessionLocked()
		} else {
			a.running = false
			a.loadSessionIntoChatLocked()
		}
	} else if !newRunActive {
		a.running = false
	}
	a.refreshHomeLocked()
	a.chatMu.Unlock()
	a.requestRedraw()
}

func (a *App) handleAttachedEvent(e session.RunEvent, mainAgentID string) {
	if a.agent == nil {
		return
	}
	if e.Type == session.RunEventAskResolved {
		a.chatMu.Lock()
		a.askAgentID = ""
		a.chatMu.Unlock()
		if a.home != nil && a.home.AskOpen() {
			a.home.ClearAsk()
			a.requestRedraw()
		}
		return
	}
	ev, ok := live.DecodeAgentEvent(e)
	if !ok {
		return
	}
	if ev.AgentID == mainAgentID {
		ev.AgentID = a.agent.ID
	}
	if ev.ParentAgentID == mainAgentID {
		ev.ParentAgentID = a.agent.ID
	}

	switch ev.Type {
	case session.AgentEventInput:
		a.onAgentInput(agent.AgentInput{
			AgentID:     ev.AgentID,
			AgentName:   ev.AgentName,
			Input:       ev.Input,
			Attachments: ev.Attachments,
		})
	case session.AgentEventStarted:
		a.onAgentStarted(agent.AgentStarted{
			AgentID:         ev.AgentID,
			AgentName:       ev.AgentName,
			DisplayName:     ev.DisplayName,
			ParentAgentID:   ev.ParentAgentID,
			Model:           llm.Model{ID: ev.Model},
			ReasoningEffort: ev.ReasoningEffort,
		})
	case session.AgentEventToken:
		a.onAgentToken(agent.AgentToken{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			Content:   ev.Content,
			Reasoning: ev.Reasoning,
		})
	case session.AgentEventMessage:
		a.onAgentMessage(agent.AgentMessage{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			Message:   ev.Message,
		})
	case session.AgentEventToolCall:
		a.onAgentToolCall(agent.AgentToolCall{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			Call:      ev.ToolCall,
		})
	case session.AgentEventToolResult:
		toolErr := error(nil)
		if ev.ToolError != "" {
			toolErr = errString(ev.ToolError)
		}
		a.onAgentToolResult(agent.AgentToolResult{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			CallID:    ev.CallID,
			ToolName:  ev.ToolName,
			Output:    ev.ToolOutput,
			Err:       toolErr,
		})
	case session.AgentEventUsage:
		a.onAgentUsage(agent.AgentUsage{AgentID: ev.AgentID, AgentName: ev.AgentName, Usage: ev.Usage})
	case session.AgentEventAsk:
		a.onAgentAsk(agent.AgentAsk{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			Questions: ev.Questions,
		})
	case session.AgentEventFinished:
		a.onAgentFinished(agent.AgentFinished{
			AgentID:      ev.AgentID,
			AgentName:    ev.AgentName,
			Output:       ev.Output,
			Usage:        ev.Usage,
			Iterations:   ev.Iterations,
			FinishReason: ev.FinishReason,
		})
	case session.AgentEventError:
		if ev.ToolError != "" {
			a.onAgentError(agent.AgentError{
				AgentID:   ev.AgentID,
				AgentName: ev.AgentName,
				Err:       errString(ev.ToolError),
			})
		}
	case session.AgentEventErrorMessage:
		a.onErrorMessage(agent.ErrorMessage{AgentID: ev.AgentID, AgentName: ev.AgentName, Message: ev.ToolError})
	case session.AgentEventErrorMessageFinish:
		a.onErrorMessageFinished(agent.ErrorMessageFinished{AgentID: ev.AgentID, AgentName: ev.AgentName})
	case session.AgentEventSubAgentNotice:
		a.onSubAgentNotification(agent.SubAgentNotification{
			ParentAgentID: ev.ParentAgentID,
			SubAgentName:  ev.SubAgentName,
			TaskIDs:       ev.TaskIDs,
			Output:        ev.SubAgentOut,
			Err:           ev.SubAgentErr,
		})
	case session.AgentEventCompaction:
		a.onCompactionFinished(compaction.Event{
			AgentID:   ev.AgentID,
			AgentName: ev.AgentName,
			Strategy:  ev.Strategy,
			Messages:  ev.Messages,
			Tokens:    ev.Tokens,
		})
	}
}

type stringError string

func (e stringError) Error() string { return string(e) }

func errString(s string) error { return stringError(s) }
