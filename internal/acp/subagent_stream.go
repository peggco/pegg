package acp

import (
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
)

type subagentEdit struct {
	path string
	old  string
}

type subagentStreamState struct {
	mu    sync.Mutex
	edits map[string]subagentEdit
}

func (st *subagentStreamState) remember(call llm.ToolCall, path string) {
	if path == "" {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.edits == nil {
		st.edits = make(map[string]subagentEdit)
	}
	st.edits[call.ID] = subagentEdit{path: path, old: string(content)}
}

func (st *subagentStreamState) take(callID string) (subagentEdit, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	edit, ok := st.edits[callID]
	delete(st.edits, callID)
	return edit, ok
}

func (s *Server) subscribeSubagentStream() {
	if s.bus == nil {
		return
	}
	s.subStreams = &subagentStreamState{edits: make(map[string]subagentEdit)}
	_ = s.bus.Subscribe(agent.TopicAgentMessage, s.handleSubagentMessage)
	_ = s.bus.Subscribe(agent.TopicAgentToolCall, s.handleSubagentToolCall)
	_ = s.bus.Subscribe(agent.TopicAgentToolResult, s.handleSubagentToolResult)
}

func (s *Server) handleSubagentMessage(ev agent.AgentMessage) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	msg := ev.Message
	if reasoning, ok := msg.Reasoning.(string); ok && strings.TrimSpace(reasoning) != "" {
		s.notify(acpSess.ID, SessionUpdate{
			SessionUpdate: "agent_thought_chunk",
			MessageID:     "thought-" + uuid.NewString(),
			AgentID:       ev.AgentID,
			AgentName:     ev.AgentName,
			Content:       &ContentBlock{Type: "text", Text: reasoning},
		})
	}
	text := llm.MessageText(msg)
	if strings.TrimSpace(text) == "" {
		return
	}
	s.notify(acpSess.ID, SessionUpdate{
		SessionUpdate: "agent_message_chunk",
		MessageID:     uuid.NewString(),
		AgentID:       ev.AgentID,
		AgentName:     ev.AgentName,
		Content:       &ContentBlock{Type: "text", Text: text},
	})
}

func (s *Server) handleSubagentToolCall(ev agent.AgentToolCall) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	call := ev.Call
	args := parseArgs(call.Function.Arguments)
	if path, ok := toolPath(args); ok && isEditTool(call.Function.Name) && s.subStreams != nil {
		s.subStreams.remember(call, absPath(acpSess.Cwd, path))
	}
	s.notify(acpSess.ID, SessionUpdate{
		SessionUpdate: "tool_call",
		ToolCallID:    call.ID,
		ToolName:      call.Function.Name,
		Title:         toolTitle(call.Function.Name, args),
		Kind:          string(toolKind(call.Function.Name)),
		Status:        string(ToolCallPending),
		RawInput:      args,
		Locations:     toolLocations(acpSess.Cwd, args),
		AgentID:       ev.AgentID,
		AgentName:     ev.AgentName,
	})
}

func (s *Server) handleSubagentToolResult(ev agent.AgentToolResult) {
	acpSess, ok := s.subagentSession(ev.AgentID)
	if !ok {
		return
	}
	status := ToolCallCompleted
	errText := ""
	if ev.Err != nil {
		status = ToolCallFailed
		errText = ev.Err.Error()
	}
	update := SessionUpdate{
		SessionUpdate: "tool_call_update",
		ToolCallID:    ev.CallID,
		ToolName:      ev.ToolName,
		Status:        string(status),
		RawOutput:     truncate(ev.Output, 128*1024),
		Error:         errText,
		AgentID:       ev.AgentID,
		AgentName:     ev.AgentName,
	}
	if status == ToolCallCompleted && s.subStreams != nil && isEditTool(ev.ToolName) {
		if edit, ok := s.subStreams.take(ev.CallID); ok {
			content, err := os.ReadFile(edit.path)
			if err == nil {
				update.Contents = []ToolCallContent{{
					Type: "diff",
					Diff: &Diff{
						Path:    edit.path,
						OldText: truncate(edit.old, 2*1024*1024),
						NewText: truncate(string(content), 2*1024*1024),
					},
				}}
				update.Locations = []ToolCallLocation{{Path: edit.path}}
			}
		}
	}
	s.notify(acpSess.ID, update)
}
