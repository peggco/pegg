package acp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
)

const (
	permissionQuestionID = "decision"
	permissionTimeout    = 10 * time.Minute
	askTimeout           = 30 * time.Minute
)

type askParams struct {
	SessionID string              `json:"sessionId"`
	AgentID   string              `json:"agentId,omitempty"`
	AgentName string              `json:"agentName,omitempty"`
	Questions []agent.AskQuestion `json:"questions"`
}

type askResult struct {
	Answers map[string]string `json:"answers"`
}

type permissionRequestParams struct {
	SessionID string             `json:"sessionId"`
	Question  string             `json:"question,omitempty"`
	ToolCall  permissionToolCall `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type permissionToolCall struct {
	ToolCallID string `json:"toolCallId,omitempty"`
	Title      string `json:"title,omitempty"`
	Kind       string `json:"kind,omitempty"`
	RawInput   any    `json:"rawInput,omitempty"`
}

func (s *Server) subscribeAsk() {
	if s.bus == nil {
		return
	}
	_ = s.bus.Subscribe(agent.TopicAgentAsk, s.handleAgentAsk)
	_ = s.bus.Subscribe(agent.TopicAgentStarted, s.handleAgentStarted)
}

func (s *Server) handleAgentStarted(ev agent.AgentStarted) {
	if ev.AgentID == "" || ev.ParentAgentID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID, ok := s.agents[ev.ParentAgentID]; ok {
		s.agents[ev.AgentID] = sessionID
	}
}

func (s *Server) handleAgentAsk(ev agent.AgentAsk) {
	acpSess, ok := s.sessionForAgent(ev.AgentID)
	if !ok {
		s.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
			AgentID: ev.AgentID,
			Answers: deniedAnswers(ev.Questions),
		})
		return
	}

	ctx, ok := acpSess.PromptContext()
	if !ok || ctx == nil {
		ctx = context.Background()
	}

	answers, err := s.askClient(ctx, acpSess, ev)
	if err != nil {
		s.log.Fdebug("acp: ask failed: %v", err)
		answers = deniedAnswers(ev.Questions)
	}
	if answers == nil {
		answers = map[string]string{}
	}
	s.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
		AgentID: ev.AgentID,
		Answers: answers,
	})
}

func (s *Server) askClient(ctx context.Context, acpSess *ACPSession, ev agent.AgentAsk) (map[string]string, error) {
	if len(ev.Questions) == 1 && isPermissionQuestion(ev.Questions[0]) {
		return s.askPermission(ctx, acpSess, ev)
	}
	return s.askQuestions(ctx, acpSess, ev)
}

func isPermissionQuestion(q agent.AskQuestion) bool {
	return q.ID == permissionQuestionID
}

func planFilePath(cwd string) string {
	if root, _, err := config.ProjectRootFor(cwd); err == nil && root != "" {
		return filepath.Join(root, "plans", "plan.md")
	}
	return filepath.Join(cwd, ".pegg", "plans", "plan.md")
}

func (s *Server) askPermission(ctx context.Context, acpSess *ACPSession, ev agent.AgentAsk) (map[string]string, error) {
	q := ev.Questions[0]
	question := q.Question
	if strings.Contains(question, "exitplanmode") {
		if data, err := os.ReadFile(planFilePath(acpSess.Cwd)); err == nil {
			question += "\n\nPlan file:\n" + truncate(string(data), 128*1024)
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, permissionTimeout)
	defer cancel()

	params := permissionRequestParams{
		SessionID: acpSess.ID,
		Question:  question,
		ToolCall: permissionToolCall{
			Title: q.Question,
			Kind:  string(ToolKindOther),
		},
		Options: []PermissionOption{
			{OptionId: "allow_once", Name: "Allow", Kind: PermAllowOnce},
			{OptionId: "allow_always", Name: "Allow All", Kind: PermAllowAlways},
			{OptionId: "reject_once", Name: "Reject", Kind: PermRejectOnce},
			{OptionId: "reject_always", Name: "Reject Always", Kind: PermRejectAlways},
		},
	}
	raw, err := s.client.call(callCtx, "session/request_permission", params, permissionTimeout)
	if err != nil {
		return nil, err
	}
	option, err := parsePermissionOutcome(raw)
	if err != nil {
		return nil, err
	}
	switch option {
	case "allow_once", "allow", "Allow":
		return map[string]string{"decision": "Allow"}, nil
	case "allow_always", "Allow All":
		return map[string]string{"decision": "Allow All"}, nil
	default:
		return map[string]string{"decision": "Reject"}, nil
	}
}

func (s *Server) askQuestions(ctx context.Context, acpSess *ACPSession, ev agent.AgentAsk) (map[string]string, error) {
	params := askParams{
		SessionID: acpSess.ID,
		AgentID:   ev.AgentID,
		AgentName: ev.AgentName,
		Questions: ev.Questions,
	}
	raw, err := s.client.call(ctx, "pegg/ask", params, askTimeout)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result askResult `json:"result"`
		Error  *RPCError `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("acp: parse ask response: %w", err)
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	if resp.Result.Answers == nil {
		return map[string]string{}, nil
	}
	return resp.Result.Answers, nil
}

func parsePermissionOutcome(raw json.RawMessage) (string, error) {
	var resp struct {
		Result struct {
			Outcome any    `json:"outcome"`
			Option  string `json:"optionId,omitempty"`
		} `json:"result"`
		Error *RPCError `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("acp: parse permission response: %w", err)
	}
	if resp.Error != nil {
		return "", resp.Error
	}
	if resp.Result.Option != "" {
		return resp.Result.Option, nil
	}
	switch v := resp.Result.Outcome.(type) {
	case string:
		return v, nil
	case map[string]any:
		if id, _ := v["optionId"].(string); id != "" {
			return id, nil
		}
		if kind, _ := v["outcome"].(string); kind != "" {
			return kind, nil
		}
	}
	return "", fmt.Errorf("acp: permission response missing outcome")
}

func deniedAnswers(questions []agent.AskQuestion) map[string]string {
	if len(questions) == 0 {
		return map[string]string{}
	}
	if isPermissionQuestion(questions[0]) {
		return map[string]string{"decision": "Reject"}
	}
	out := make(map[string]string, len(questions))
	for _, q := range questions {
		if q.Type == "boolean" {
			out[q.ID] = "false"
			continue
		}
		out[q.ID] = ""
	}
	return out
}
