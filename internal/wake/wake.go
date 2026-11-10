package wake

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

const (
	retryDelay = 100 * time.Millisecond
	retryMax   = 50
)

type Deps struct {
	Bus      event.Bus
	Runs     *runpkg.Coordinator
	Sessions *session.Manager
	Log      *logger.Logger
}

type Waker struct {
	deps   Deps
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	inflight map[string]bool
	parents  map[string]*agent.Agent

	onNotify func(agent.SubAgentNotification)
	onRun    func(session.RunFinished)
}

func New(deps Deps) *Waker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Waker{
		deps:     deps,
		ctx:      ctx,
		cancel:   cancel,
		inflight: make(map[string]bool),
		parents:  make(map[string]*agent.Agent),
	}
	w.onNotify = w.onNotification
	w.onRun = w.onRunFinished
	return w
}

func (w *Waker) Start() error {
	if w == nil || w.deps.Bus == nil {
		return nil
	}
	if err := w.deps.Bus.Subscribe(agent.TopicSubAgentNotification, w.onNotify); err != nil {
		return err
	}
	return w.deps.Bus.Subscribe(session.TopicRunFinished, w.onRun)
}

func (w *Waker) Stop() {
	if w == nil {
		return
	}
	if w.deps.Bus != nil {
		_ = w.deps.Bus.Unsubscribe(agent.TopicSubAgentNotification, w.onNotify)
		_ = w.deps.Bus.Unsubscribe(session.TopicRunFinished, w.onRun)
	}
	w.cancel()
}

func (w *Waker) onNotification(e agent.SubAgentNotification) {
	parent := e.Parent
	if parent == nil || w.deps.Runs == nil || w.deps.Sessions == nil {
		return
	}
	sessionID := e.SessionID
	if sessionID == "" {
		sessionID = parent.SessionID()
	}
	if sessionID == "" {
		return
	}
	w.mu.Lock()
	w.parents[parent.ID] = parent
	w.mu.Unlock()
	w.enqueue(parent, sessionID)
}

func (w *Waker) onRunFinished(e session.RunFinished) {
	if w.deps.Runs == nil || w.deps.Sessions == nil {
		return
	}
	w.mu.Lock()
	parent := w.parents[e.AgentID]
	w.mu.Unlock()
	if parent == nil || !parent.HasPendingNotifications() {
		return
	}
	sessionID := e.SessionID
	if sessionID == "" {
		sessionID = parent.SessionID()
	}
	if sessionID == "" {
		return
	}
	w.enqueue(parent, sessionID)
}

func (w *Waker) enqueue(parent *agent.Agent, sessionID string) {
	key := parent.ID + "\x00" + sessionID

	w.mu.Lock()
	if w.inflight[key] {
		w.mu.Unlock()
		return
	}
	w.inflight[key] = true
	w.mu.Unlock()

	go func() {
		clean := w.wake(parent, sessionID)
		w.mu.Lock()
		delete(w.inflight, key)
		pending := parent.HasPendingNotifications()
		if clean && !pending {
			delete(w.parents, parent.ID)
		}
		w.mu.Unlock()
		if clean && pending {
			w.enqueue(parent, sessionID)
		}
	}()
}

func (w *Waker) wake(parent *agent.Agent, sessionID string) bool {
	for attempt := 0; attempt < retryMax; attempt++ {
		if w.ctx.Err() != nil {
			return false
		}
		if !parent.HasPendingNotifications() {
			return true
		}
		sess, err := w.deps.Sessions.LatestInChain(sessionID)
		if err != nil || sess == nil {
			return false
		}
		sessionID = sess.ID
		if active, aerr := w.deps.Runs.Active(sessionID); aerr == nil && active != nil && active.State == session.RunStateRunning {
			select {
			case <-w.ctx.Done():
				return false
			case <-time.After(retryDelay):
			}
			continue
		}
		continued, busy := w.continueAgent(parent, sessionID)
		if busy {
			select {
			case <-w.ctx.Done():
				return false
			case <-time.After(retryDelay):
			}
			continue
		}
		if !continued {
			return false
		}
	}
	return false
}

func (w *Waker) continueAgent(parent *agent.Agent, sessionID string) (bool, bool) {
	msgs, err := w.deps.Sessions.Messages(sessionID)
	if err != nil {
		return false, false
	}
	history := make([]llm.Message, 0, len(msgs)+1)
	if parent.SystemPrompt != "" {
		history = append(history, llm.SystemMessage(parent.SystemPrompt))
	}
	history = append(history, session.MessagesToLLM(msgs)...)

	lease, err := w.deps.Runs.Claim(w.ctx, runpkg.ClaimRequest{
		SessionID: sessionID,
		AgentID:   parent.ID,
		AgentName: parent.Name,
		Provider:  providerName(parent),
		Model:     parent.Model.ID,
		BaseSeq:   w.deps.Runs.BaseSeq(sessionID),
	})
	if err != nil {
		var busy *runpkg.BusyError
		if errors.As(err, &busy) {
			return false, true
		}
		w.debugf("wake: claim session %s: %v", sessionID, err)
		return false, false
	}

	parent.SetSessionID(sessionID)
	if w.deps.Bus != nil {
		w.deps.Bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   parent.ID,
			SessionID: sessionID,
		})
	}

	if _, err := w.deps.Runs.Continue(lease.Context(), lease, parent, history); err != nil {
		w.debugf("wake: continue session %s: %v", sessionID, err)
		if parent.HasPendingNotifications() {
			return false, false
		}
	}
	return true, false
}

func (w *Waker) debugf(format string, args ...any) {
	if w.deps.Log == nil {
		return
	}
	w.deps.Log.Fdebug(format, args...)
}

func providerName(a *agent.Agent) string {
	if a == nil || a.Provider == nil {
		return ""
	}
	return a.Provider.Name()
}
