package acp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/google/uuid"
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

type SessionPromptParams struct {
	SessionId SessionId      `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type SessionPromptResult struct {
	StopReason StopReason `json:"stopReason"`
	Usage      *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	Used int   `json:"used"`
	Size int   `json:"size"`
	Cost *Cost `json:"cost,omitempty"`
}

type promptState struct {
	sessionID string
	textIDs   map[string]string
	thoughtID map[string]string
	oldFiles  map[string]string
}

func (s *Server) handleSessionPrompt(ctx context.Context, params json.RawMessage, id any) (any, error) {
	var req SessionPromptParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid session/prompt params"}
	}

	acpSess, ok := s.getSession(string(req.SessionId))
	if !ok {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "session not found"}
	}
	input, attachments, err := s.preparePrompt(ctx, acpSess, req.Prompt)
	if err != nil {
		return nil, err
	}
	if input == "" && len(attachments) == 0 {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "prompt contains no text, audio, or attachments"}
	}
	if input == "" {
		input = "Please analyze the attached content."
	}

	promptCtx, cancel := context.WithCancel(ctx)
	if !acpSess.StartPrompt(promptCtx, cancel, id) {
		cancel()
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "a prompt is already running for this session"}
	}
	defer acpSess.ClearPrompt()

	if acpSess.Agent != nil {
		acpSess.Agent.Attachments = attachments
	}

	var history []llm.Message
	if acpSess.PeggSessionID != "" {
		if msgs, err := s.sessions.Messages(acpSess.PeggSessionID); err == nil && len(msgs) > 0 {
			if acpSess.Agent.SystemPrompt != "" {
				history = append(history, llm.SystemMessage(acpSess.Agent.SystemPrompt))
			}
			history = append(history, session.MessagesToLLM(msgs)...)
			s.bus.Publish(session.TopicSessionResume, session.SessionResume{
				AgentID:   acpSess.Agent.ID,
				SessionID: acpSess.PeggSessionID,
			})
		}
	}

	state := &promptState{
		sessionID: string(req.SessionId),
		textIDs:   make(map[string]string),
		thoughtID: make(map[string]string),
		oldFiles:  make(map[string]string),
	}

	eventCh := make(chan agent.StreamEvent, 256)
	go func() {
		for ev := range eventCh {
			s.handleStreamEvent(state, acpSess, ev)
		}
	}()

	stream := func(ev agent.StreamEvent) error {
		select {
		case eventCh <- ev:
		case <-promptCtx.Done():
			return promptCtx.Err()
		}
		return nil
	}
	var result *agent.RunResult
	var runErr error
	if len(history) > 0 {
		result, runErr = acpSess.Agent.ResumeStream(promptCtx, input, history, stream)
	} else {
		result, runErr = acpSess.Agent.RunStream(promptCtx, input, stream)
	}

	close(eventCh)
	if acpSess.Agent != nil {
		acpSess.Agent.Attachments = nil
	}

	if result != nil && result.Usage.TotalTokens > 0 {
		s.notify(state.sessionID, SessionUpdate{
			SessionUpdate: "usage_update",
			Used:          result.Usage.TotalTokens,
			Size:          200000,
			Cost:          &Cost{Amount: result.Usage.Cost, Currency: "USD"},
		})
	}

	stopReason := StopReasonEndTurn
	if runErr != nil {
		if promptCtx.Err() != nil {
			stopReason = StopReasonCancelled
		} else {
			stopReason = StopReasonError
			s.notify(state.sessionID, SessionUpdate{
				SessionUpdate: "error",
				Error:         runErr.Error(),
			})
		}
	}

	out := SessionPromptResult{StopReason: stopReason}
	if result != nil {
		out.Usage = &UsageInfo{
			Used: result.Usage.TotalTokens,
			Size: 200000,
			Cost: &Cost{Amount: result.Usage.Cost, Currency: "USD"},
		}
	}
	return out, nil
}

func (s *Server) preparePrompt(ctx context.Context, acpSess *ACPSession, blocks []ContentBlock) (string, []llm.Attachment, error) {
	var parts []string
	var attachments []llm.Attachment

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) != "" {
				parts = append(parts, b.Text)
			}
		case "image":
			if b.Data == "" {
				continue
			}
			attachments = append(attachments, llm.NewImageAttachmentFromBase64(defaultMime(b.MimeType, "image/png"), b.Data))
		case "video":
			if b.Data == "" {
				continue
			}
			attachments = append(attachments, llm.NewVideoAttachmentFromBase64(defaultMime(b.MimeType, "video/mp4"), b.Data))
		case "audio":
			if b.Data != "" {
				attachments = append(attachments, llm.NewAudioAttachmentFromBase64(defaultMime(b.MimeType, "audio/webm"), b.Data))
				continue
			}
			text, err := s.transcribeAudioBlocks(ctx, []ContentBlock{b})
			if err != nil {
				return "", nil, err
			}
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		case "resource":
			if b.Resource == nil {
				continue
			}
			r := b.Resource
			if r.Text != "" {
				name := r.URI
				if name == "" {
					name = "embedded resource"
				}
				parts = append(parts, fmt.Sprintf("<file path=%q>\n%s\n</file>", name, r.Text))
				continue
			}
			if r.Blob != "" {
				attachments = append(attachments, llm.NewFileAttachmentFromBase64(defaultMime(r.MimeType, "application/octet-stream"), r.Blob, resourceName(r.URI)))
			}
		case "resource_link":
			if b.URI != "" {
				parts = append(parts, fmt.Sprintf("Referenced file: %s", b.URI))
			}
		}
	}

	return strings.Join(parts, "\n\n"), attachments, nil
}

func defaultMime(got, fallback string) string {
	if strings.TrimSpace(got) == "" {
		return fallback
	}
	return got
}

func resourceName(uri string) string {
	if uri == "" {
		return "attachment"
	}
	name := filepath.Base(uri)
	if name == "." || name == "/" {
		return "attachment"
	}
	return name
}

func (s *Server) handleStreamEvent(st *promptState, acpSess *ACPSession, ev agent.StreamEvent) {
	agentID := ev.AgentID
	agentName := ev.AgentName
	if agentID == "" {
		agentID = "orchestrator"
	}

	switch ev.Type {
	case agent.StreamReasoning:
		if strings.TrimSpace(ev.Reasoning) == "" {
			return
		}
		id := st.thoughtID[agentID]
		if id == "" {
			id = "thought-" + uuid.NewString()
			st.thoughtID[agentID] = id
		}
		s.notify(st.sessionID, SessionUpdate{
			SessionUpdate: "agent_thought_chunk",
			MessageID:     id,
			AgentID:       agentID,
			AgentName:     agentName,
			Content:       &ContentBlock{Type: "text", Text: ev.Reasoning},
		})
	case agent.StreamToken:
		if ev.Content == "" {
			return
		}
		id := st.textIDs[agentID]
		if id == "" {
			id = uuid.NewString()
			st.textIDs[agentID] = id
		}
		s.notify(st.sessionID, SessionUpdate{
			SessionUpdate: "agent_message_chunk",
			MessageID:     id,
			AgentID:       agentID,
			AgentName:     agentName,
			Content:       &ContentBlock{Type: "text", Text: ev.Content},
		})
	case agent.StreamToolCall:
		if ev.ToolCall == nil {
			return
		}
		call := ev.ToolCall
		args := parseArgs(call.Function.Arguments)
		delete(st.textIDs, agentID)
		delete(st.thoughtID, agentID)
		if path, ok := toolPath(args); ok && isEditTool(call.Function.Name) {
			if content, err := os.ReadFile(absPath(acpSess.Cwd, path)); err == nil {
				st.oldFiles[call.ID] = string(content)
			}
		}
		s.notify(st.sessionID, SessionUpdate{
			SessionUpdate: "tool_call",
			ToolCallID:    call.ID,
			ToolName:      call.Function.Name,
			Title:         toolTitle(call.Function.Name, args),
			Kind:          string(toolKind(call.Function.Name)),
			Status:        string(ToolCallPending),
			RawInput:      args,
			Locations:     toolLocations(acpSess.Cwd, args),
			AgentID:       agentID,
			AgentName:     agentName,
		})
		if call.Function.Name == "todowrite" {
			if entries, ok := planEntries(args); ok {
				s.notify(st.sessionID, SessionUpdate{
					SessionUpdate: "plan",
					Entries:       entries,
					AgentID:       agentID,
					AgentName:     agentName,
				})
			}
		}
	case agent.StreamToolResult:
		if ev.ToolCall == nil {
			return
		}
		call := ev.ToolCall
		args := parseArgs(call.Function.Arguments)
		status := ToolCallCompleted
		errText := ""
		if ev.ToolErr != nil {
			status = ToolCallFailed
			errText = ev.ToolErr.Error()
		}
		update := SessionUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    call.ID,
			ToolName:      call.Function.Name,
			Status:        string(status),
			RawOutput:     truncate(ev.ToolOutput, 128*1024),
			Error:         errText,
			AgentID:       agentID,
			AgentName:     agentName,
		}
		if status == ToolCallCompleted {
			if path, ok := toolPath(args); ok && isEditTool(call.Function.Name) {
				newContent, _ := os.ReadFile(absPath(acpSess.Cwd, path))
				oldContent, hadOld := st.oldFiles[call.ID]
				if hadOld || len(newContent) > 0 {
					update.Contents = []ToolCallContent{{
						Type: "diff",
						Diff: &Diff{
							Path:    path,
							OldText: truncate(oldContent, 2*1024*1024),
							NewText: truncate(string(newContent), 2*1024*1024),
						},
					}}
					update.Locations = []ToolCallLocation{{Path: absPath(acpSess.Cwd, path)}}
				}
			}
		}
		s.notify(st.sessionID, update)
	case agent.StreamCompaction:
		s.notify(st.sessionID, SessionUpdate{
			SessionUpdate: "agent_message_chunk",
			MessageID:     uuid.NewString(),
			AgentID:       agentID,
			AgentName:     agentName,
			Content: &ContentBlock{Type: "text",
				Text: fmt.Sprintf("[Context compacted (%s) — %d messages, %d tokens]",
					ev.Strategy, ev.Messages, ev.Tokens)},
		})
	}
}

func extractPromptText(blocks []ContentBlock) string {
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			return b.Text
		}
	}
	return ""
}

func parseArgs(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]any{"raw": raw}
	}
	return out
}

func stringArg(args map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func toolPath(args map[string]any) (string, bool) {
	p := stringArg(args, "filePath", "file_path", "path", "file", "target")
	if p == "" {
		return "", false
	}
	return p, true
}

func absPath(cwd, p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if cwd == "" {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

func isEditTool(name string) bool {
	switch name {
	case "write", "edit":
		return true
	default:
		return false
	}
}

func toolKind(name string) ToolKind {
	switch name {
	case "read", "list":
		return ToolKindRead
	case "write", "edit":
		return ToolKindEdit
	case "glob", "grep":
		return ToolKindSearch
	case "bash":
		return ToolKindExecute
	case "webfetch", "websearch":
		return ToolKindFetch
	case "task", "taskstatus", "todowrite", "todoread":
		return ToolKindThink
	case "enterplanmode", "exitplanmode":
		return ToolKindSwitch
	default:
		return ToolKindOther
	}
}

func toolTitle(name string, args map[string]any) string {
	switch name {
	case "bash":
		if cmd := stringArg(args, "command"); cmd != "" {
			return "bash: " + truncate(cmd, 120)
		}
	case "read", "write", "edit", "list":
		if p := stringArg(args, "filePath", "file_path", "path"); p != "" {
			return name + ": " + p
		}
	case "grep", "glob":
		if p := stringArg(args, "pattern", "query"); p != "" {
			return name + ": " + p
		}
	case "task":
		if p := stringArg(args, "name", "description"); p != "" {
			return "task: " + p
		}
	case "webfetch", "websearch":
		if p := stringArg(args, "url", "query"); p != "" {
			return name + ": " + truncate(p, 120)
		}
	}
	return name
}

func toolLocations(cwd string, args map[string]any) []ToolCallLocation {
	p, ok := toolPath(args)
	if !ok {
		return nil
	}
	line := 0
	if v, ok := args["line"].(float64); ok {
		line = int(v)
	}
	return []ToolCallLocation{{Path: absPath(cwd, p), Line: line}}
}

func planEntries(args map[string]any) ([]PlanEntry, bool) {
	raw, ok := args["todos"]
	if !ok {
		return nil, false
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]PlanEntry, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		title, _ := m["title"].(string)
		status, _ := m["status"].(string)
		priority, _ := m["priority"].(string)
		if title == "" {
			continue
		}
		out = append(out, PlanEntry{Content: title, Status: status, Priority: priority})
	}
	return out, true
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n…[truncated]"
}
