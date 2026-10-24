package connect

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

type StartParams struct {
	Message         string
	SessionID       string
	Provider        string
	Model           string
	ReasoningEffort string
	Mode            string
	Files           []string
	Attachments     []llm.Attachment
}

func resolveRunMode(provided, sessionMode string) agent.Mode {
	if m := agent.Mode(provided); m.Valid() {
		return m
	}
	return agent.NormalizeMode(agent.Mode(sessionMode))
}

func (m *runManager) Start(p StartParams) (RunInfo, error) {
	if strings.TrimSpace(p.Message) == "" && len(p.Files) == 0 && len(p.Attachments) == 0 {
		return RunInfo{}, &FrameError{Code: ErrCodeBadRequest, Message: "message is required"}
	}
	if m.c.deps.Sessions == nil {
		return RunInfo{}, &FrameError{Code: ErrCodeUnavailable, Message: "session store unavailable"}
	}
	if !m.tryAcquire() {
		return RunInfo{}, &FrameError{Code: ErrCodeConflict, Message: "maximum concurrent runs reached"}
	}
	acquired := true
	defer func() {
		if acquired {
			select {
			case <-m.sem:
			default:
			}
		}
	}()

	orch, err := agents.New("orchestrator")
	if err != nil {
		return RunInfo{}, &FrameError{Code: ErrCodeInternal, Message: "create agent: " + err.Error()}
	}
	orch.Bus = m.c.deps.Bus

	runID := newRunID()

	prov, mdl, err := m.resolveModel(p.Provider, p.Model)
	if err != nil {
		return RunInfo{}, err
	}
	orch.SetModelProvider(mdl, &traceProvider{inner: prov, trace: m.c.trace, runID: runID})
	m.c.trace("run %s starting on %s/%s", shortID(runID), prov.Name(), mdl.ID)
	if p.ReasoningEffort != "" {
		orch.ReasoningEffort = p.ReasoningEffort
	}

	if len(p.Files) > 0 || len(p.Attachments) > 0 {
		atts, aerr := prepareAttachments(p.Attachments, p.Files)
		if aerr != nil {
			return RunInfo{}, &FrameError{Code: ErrCodeBadRequest, Message: aerr.Error()}
		}
		if len(atts) > 0 {
			orch.Attachments = atts
		}
	}

	var (
		sessionID string
		history   []llm.Message
		mode      = resolveRunMode(p.Mode, "")
	)
	if p.SessionID != "" {
		existing, gerr := m.c.deps.Sessions.Get(p.SessionID)
		if gerr != nil {
			return RunInfo{}, &FrameError{Code: ErrCodeNotFound, Message: "session not found: " + p.SessionID}
		}
		msgs, merr := m.c.deps.Sessions.Messages(p.SessionID)
		if merr != nil {
			return RunInfo{}, &FrameError{Code: ErrCodeNotFound, Message: "session not found: " + p.SessionID}
		}
		sessionID = p.SessionID
		mode = resolveRunMode(p.Mode, existing.Mode)
		if string(mode) != existing.Mode {
			if serr := m.c.deps.Sessions.SetMode(sessionID, string(mode)); serr != nil {
				return RunInfo{}, &FrameError{Code: ErrCodeInternal, Message: "set session mode: " + serr.Error()}
			}
		}
		if orch.SystemPrompt != "" {
			history = append(history, llm.SystemMessage(orch.SystemPrompt))
		}
		history = append(history, session.MessagesToLLM(msgs)...)
	} else {
		s, serr := m.c.deps.Sessions.Create(session.CreateOptions{
			Provider:        prov.Name(),
			Model:           mdl.ID,
			ReasoningEffort: orch.ReasoningEffort,
			Mode:            string(mode),
			ProjectDir:      m.c.workspace(),
		})
		if serr != nil {
			return RunInfo{}, &FrameError{Code: ErrCodeInternal, Message: "create session: " + serr.Error()}
		}
		sessionID = s.ID
	}

	orch.SetMode(mode)

	runCtx, cancel := context.WithCancel(m.base)
	var lease *runpkg.Lease
	if m.c.deps.Runs != nil {
		lease, err = m.c.deps.Runs.Claim(runCtx, runpkg.ClaimRequest{
			SessionID: sessionID,
			AgentID:   orch.ID,
			AgentName: orch.Name,
			Provider:  prov.Name(),
			Model:     mdl.ID,
			BaseSeq:   m.c.deps.Runs.BaseSeq(sessionID),
		})
		if err != nil {
			cancel()
			var busy *runpkg.BusyError
			if errors.As(err, &busy) {
				return RunInfo{}, &FrameError{Code: ErrCodeConflict, Message: "session is already running in another client"}
			}
			return RunInfo{}, &FrameError{Code: ErrCodeInternal, Message: "claim session: " + err.Error()}
		}
		runCtx = lease.Context()
		runID = lease.ID()
	}
	r := &run{
		id:        runID,
		sessionID: sessionID,
		agentID:   orch.ID,
		provider:  prov.Name(),
		model:     mdl.ID,
		state:     runStateRunning,
		startedAt: time.Now(),
		cancel:    cancel,
		lease:     lease,
	}
	m.register(r)
	acquired = false

	if m.c.deps.Bus != nil {
		m.c.deps.Bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   orch.ID,
			SessionID: sessionID,
		})
	}

	go m.execute(runCtx, r, orch, p.Message, history)
	return r.snapshot(), nil
}
