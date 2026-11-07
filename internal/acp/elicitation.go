package acp

import (
	"context"
	"fmt"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
)

func (s *Server) clientSupportsElicitation(mode string) bool {
	caps := s.clientCapabilities()
	if caps == nil || caps.Elicitation == nil {
		return false
	}
	switch mode {
	case "form":
		return caps.Elicitation.Form != nil
	case "url":
		return caps.Elicitation.URL != nil
	}
	return false
}

func (s *Server) Elicit(ctx context.Context, params ElicitationCreateParams) (*ElicitationCreateResult, error) {
	raw, err := s.client.call(ctx, "elicitation/create", params, 0)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result ElicitationCreateResult `json:"result"`
		Error  *RPCError               `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return &resp.Result, nil
}

func (s *Server) CompleteElicitation(elicitationID ElicitationId) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "elicitation/complete",
		"params":  map[string]any{"elicitationId": elicitationID},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.sendRaw(raw)
}

func askToElicitation(sessionID string, ev agent.AgentAsk) ElicitationCreateParams {
	props := map[string]any{}
	var required []string
	for _, q := range ev.Questions {
		props[q.ID] = questionSchema(q)
		if q.Required {
			required = append(required, q.ID)
		}
	}
	return ElicitationCreateParams{
		Mode:      "form",
		Message:   askMessage(ev),
		SessionID: SessionId(sessionID),
		RequestedSchema: &ElicitationSchema{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
	}
}

func questionSchema(q agent.AskQuestion) map[string]any {
	switch q.Type {
	case "boolean":
		return map[string]any{"type": "boolean", "title": q.Question}
	case "select":
		return map[string]any{"type": "string", "title": q.Question, "enum": q.Options}
	case "multiselect":
		return map[string]any{"type": "array", "title": q.Question, "items": map[string]any{"type": "string", "enum": q.Options}}
	default:
		return map[string]any{"type": "string", "title": q.Question}
	}
}

func askMessage(ev agent.AgentAsk) string {
	if len(ev.Questions) == 1 {
		return ev.Questions[0].Question
	}
	return "The agent needs more information."
}

func elicitationAnswers(questions []agent.AskQuestion, content any) map[string]string {
	out := map[string]string{}
	m, _ := content.(map[string]any)
	for _, q := range questions {
		v, ok := m[q.ID]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			out[q.ID] = val
		case bool:
			out[q.ID] = fmt.Sprintf("%t", val)
		case float64:
			out[q.ID] = fmt.Sprintf("%v", val)
		case []any:
			parts := make([]string, 0, len(val))
			for _, item := range val {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			out[q.ID] = strings.Join(parts, ", ")
		}
	}
	return out
}
