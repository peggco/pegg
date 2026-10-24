package connect

import (
	"context"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func (c *Connector) registerChatMethods() {
	c.dispatch.register("chat.send", ScopeChat, c.handleChatSend)
	c.dispatch.register("run.cancel", ScopeChat, c.handleRunCancel)
	c.dispatch.register("run.answer", ScopeChat, c.handleRunAnswer)
	c.dispatch.register("run.list", ScopeRead, c.handleRunList)
}

type chatSendParams struct {
	Message         string           `json:"message"`
	SessionID       string           `json:"session_id"`
	Provider        string           `json:"provider"`
	Model           string           `json:"model"`
	ReasoningEffort string           `json:"reasoning_effort"`
	Mode            string           `json:"mode"`
	Files           []string         `json:"files"`
	Attachments     []llm.Attachment `json:"attachments"`
}

func (c *Connector) handleChatSend(_ context.Context, req *Request) (any, error) {
	var p chatSendParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	if p.SessionID == "" {
		p.SessionID = req.SessionID
	}
	info, err := c.runs.Start(StartParams{
		Message:         p.Message,
		SessionID:       p.SessionID,
		Provider:        p.Provider,
		Model:           p.Model,
		ReasoningEffort: p.ReasoningEffort,
		Mode:            p.Mode,
		Files:           p.Files,
		Attachments:     p.Attachments,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"run_id":     info.ID,
		"session_id": info.SessionID,
		"state":      info.State,
	}, nil
}

type runRefParams struct {
	RunID string `json:"run_id"`
}

func (c *Connector) handleRunCancel(_ context.Context, req *Request) (any, error) {
	var p runRefParams
	_ = req.Decode(&p)
	id := p.RunID
	if id == "" {
		id = req.RunID
	}
	if id == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "run_id is required"}
	}
	if err := c.runs.Cancel(id); err != nil {
		return nil, err
	}
	return map[string]any{"run_id": id, "state": runStateCanceled}, nil
}

type runAnswerParams struct {
	RunID   string            `json:"run_id"`
	Answers map[string]string `json:"answers"`
}

func (c *Connector) handleRunAnswer(_ context.Context, req *Request) (any, error) {
	var p runAnswerParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	id := p.RunID
	if id == "" {
		id = req.RunID
	}
	if id == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "run_id is required"}
	}
	if len(p.Answers) == 0 {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "answers are required"}
	}
	if err := c.runs.Answer(id, p.Answers); err != nil {
		return nil, err
	}
	Audit("run.answer", map[string]any{"run_id": id})
	return map[string]any{"run_id": id, "answered": true}, nil
}

func (c *Connector) handleRunList(_ context.Context, _ *Request) (any, error) {
	return map[string]any{"runs": c.runs.List()}, nil
}

type runAttachParams struct {
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
	After     int64  `json:"after"`
}

func (c *Connector) handleRunAttach(_ context.Context, req *Request) (any, error) {
	var p runAttachParams
	_ = req.Decode(&p)
	if p.RunID == "" {
		p.RunID = req.RunID
	}
	if p.SessionID == "" {
		p.SessionID = req.SessionID
	}
	if c.deps.Runs == nil {
		return nil, &FrameError{Code: ErrCodeUnavailable, Message: "run store unavailable"}
	}
	var run *session.Run
	var err error
	if p.RunID != "" {
		run, err = c.deps.Runs.Get(p.RunID)
	} else if p.SessionID != "" {
		run, err = c.deps.Runs.Active(p.SessionID)
	}
	if err != nil || run == nil {
		return nil, &FrameError{Code: ErrCodeNotFound, Message: "run not found"}
	}
	info := runInfoFromRun(run)
	c.enrichRun(&info)
	return info, nil
}
