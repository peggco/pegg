package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func (c *CLI) attachSession(ctx context.Context, out io.Writer, in io.Reader, sessionID string, follow bool) error {
	if c.runCoord == nil {
		return fmt.Errorf("cli: shared run store unavailable")
	}
	run, err := c.runCoord.Active(sessionID)
	if err != nil && !follow {
		return fmt.Errorf("cli: session is not running")
	}

	attachBus := event.New()
	mainID := ""
	if run != nil {
		mainID = run.AgentID
	}
	renderer := newRunRenderer(out, in, mainID, true, true)
	renderer.mainID = mainID
	renderer.bus = attachBus
	renderer.ctx = ctx
	if err := renderer.subscribe(attachBus); err != nil {
		return err
	}
	defer renderer.unsubscribe(attachBus)

	answer := func(e agent.AgentAskAnswer) {
		_ = c.runCoord.Answer(run.ID, e.Answers)
	}
	_ = attachBus.Subscribe(agent.TopicAgentAskAnswer, answer)
	defer attachBus.Unsubscribe(agent.TopicAgentAskAnswer, answer)

	if run != nil {
		renderer.write("attached to run %s (session %s)\n", run.ID, run.SessionID)
	} else {
		renderer.write("following session %s\n", sessionID)
	}

	if c.liveHub != nil {
		snapshot, since, serr := c.liveHub.SnapshotSession(sessionID, true)
		if serr != nil {
			return fmt.Errorf("cli: attach snapshot: %w", serr)
		}
		sub := c.liveHub.SubscribeSession(sessionID, true, since)
		defer sub.Close()
		for i := range snapshot {
			if snapshot[i].Type == live.EventRunEvent {
				publishAttachedEvent(attachBus, snapshot[i].Journal)
			}
			if snapshot[i].Type == live.EventRunFinished && !follow {
				return nil
			}
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case e, ok := <-sub.C:
				if !ok {
					return nil
				}
				switch e.Type {
				case live.EventRunEvent:
					publishAttachedEvent(attachBus, e.Journal)
				case live.EventRunFinished:
					if !follow {
						return nil
					}
				}
			}
		}
	}

	if run == nil {
		return fmt.Errorf("cli: live hub unavailable and no active run")
	}
	err = c.runCoord.Watch(ctx, run.ID, 0, func(e session.RunEvent) error {
		publishAttachedEvent(attachBus, e)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("cli: attach: %w", err)
	}
	return nil
}

func publishAttachedEvent(bus event.Bus, e session.RunEvent) {
	if e.Type == session.RunEventAskResolved {
		return
	}
	ev, ok := live.DecodeAgentEvent(e)
	if !ok {
		return
	}
	switch ev.Type {
	case session.AgentEventInput:
		bus.Publish(agent.TopicAgentInput, agent.AgentInput{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Input: ev.Input, Attachments: ev.Attachments,
		})
	case session.AgentEventStarted:
		bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
			AgentID: ev.AgentID, AgentName: ev.AgentName, DisplayName: ev.DisplayName,
			ParentAgentID: ev.ParentAgentID, Model: llm.Model{ID: ev.Model},
		})
	case session.AgentEventToken:
		bus.Publish(agent.TopicAgentToken, agent.AgentToken{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Content: ev.Content, Reasoning: ev.Reasoning,
		})
	case session.AgentEventMessage:
		bus.Publish(agent.TopicAgentMessage, agent.AgentMessage{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Message: ev.Message,
		})
	case session.AgentEventToolCall:
		bus.Publish(agent.TopicAgentToolCall, agent.AgentToolCall{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Call: ev.ToolCall,
		})
	case session.AgentEventToolResult:
		toolErr := error(nil)
		if ev.ToolError != "" {
			toolErr = fmt.Errorf("%s", ev.ToolError)
		}
		bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
			AgentID: ev.AgentID, AgentName: ev.AgentName, CallID: ev.CallID, ToolName: ev.ToolName,
			Output: ev.ToolOutput, Err: toolErr,
		})
	case session.AgentEventUsage:
		bus.Publish(agent.TopicAgentUsage, agent.AgentUsage{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Usage: ev.Usage,
		})
	case session.AgentEventAsk:
		bus.Publish(agent.TopicAgentAsk, agent.AgentAsk{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Questions: ev.Questions,
		})
	case session.AgentEventFinished:
		bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Output: ev.Output, Usage: ev.Usage,
			Iterations: ev.Iterations, FinishReason: ev.FinishReason,
		})
	case session.AgentEventError:
		if ev.ToolError != "" {
			bus.Publish(agent.TopicAgentError, agent.AgentError{
				AgentID: ev.AgentID, AgentName: ev.AgentName, Err: fmt.Errorf("%s", ev.ToolError),
			})
		}
	case session.AgentEventErrorMessage:
		bus.Publish(agent.TopicErrorMessage, agent.ErrorMessage{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Message: ev.ToolError,
		})
	case session.AgentEventErrorMessageFinish:
		bus.Publish(agent.TopicErrorMessageFinished, agent.ErrorMessageFinished{
			AgentID: ev.AgentID, AgentName: ev.AgentName,
		})
	case session.AgentEventSubAgentNotice:
		bus.Publish(agent.TopicSubAgentNotification, agent.SubAgentNotification{
			ParentAgentID: ev.ParentAgentID, SubAgentName: ev.SubAgentName,
			TaskIDs: ev.TaskIDs, Output: ev.SubAgentOut, Err: ev.SubAgentErr,
		})
	case session.AgentEventCompaction:
		bus.Publish(compaction.TopicCompactionFinished, compaction.Event{
			AgentID: ev.AgentID, AgentName: ev.AgentName, Strategy: ev.Strategy,
			Messages: ev.Messages, Tokens: ev.Tokens,
		})
	}
}
