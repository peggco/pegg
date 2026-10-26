package live

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/session"
)

type ToolSummary struct {
	CallID   string
	ToolName string
	Args     string
	Output   string
	Error    string
	Done     bool
}

type Summary struct {
	Content   string
	Reasoning string
	Tools     []ToolSummary
	Questions []agent.AskQuestion
	EventSeq  int64
}

func (h *Hub) SummarizeRun(runID, agentID string) (Summary, error) {
	var (
		out   Summary
		tools = make(map[string]*ToolSummary)
		order []string
		last  int64
	)
	for {
		events, err := h.coord.Events(runID, last, pageSize)
		if err != nil {
			return out, err
		}
		for _, e := range events {
			last = e.Seq
			if agentID != "" && e.AgentID != "" && e.AgentID != agentID {
				continue
			}
			ev, ok := DecodeAgentEvent(e)
			if !ok {
				continue
			}
			switch ev.Type {
			case session.AgentEventToken:
				out.Content += ev.Content
				out.Reasoning += ev.Reasoning
			case session.AgentEventToolCall:
				t := tools[ev.CallID]
				if t == nil {
					t = &ToolSummary{CallID: ev.CallID}
					tools[ev.CallID] = t
					order = append(order, ev.CallID)
				}
				t.ToolName = ev.ToolName
				t.Args = ev.ToolCall.Function.Arguments
			case session.AgentEventToolResult:
				t := tools[ev.CallID]
				if t == nil {
					t = &ToolSummary{CallID: ev.CallID}
					tools[ev.CallID] = t
					order = append(order, ev.CallID)
				}
				t.ToolName = ev.ToolName
				t.Output = ev.ToolOutput
				t.Error = ev.ToolError
				t.Done = true
			case session.AgentEventAsk:
				out.Questions = ev.Questions
			case session.RunEventAskResolved:
				out.Questions = nil
			}
		}
		if len(events) < pageSize {
			break
		}
	}
	out.EventSeq = last
	out.Tools = make([]ToolSummary, 0, len(order))
	for _, id := range order {
		out.Tools = append(out.Tools, *tools[id])
	}
	return out, nil
}
