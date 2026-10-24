package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

const (
	heartbeatInterval = 1 * time.Second
	commandInterval   = 250 * time.Millisecond
	watchInterval     = 100 * time.Millisecond
	StaleAfter        = 15 * time.Second
)

type BusyError struct {
	Run *session.Run
}

func (e *BusyError) Error() string {
	if e == nil || e.Run == nil {
		return session.ErrRunBusy.Error()
	}
	return fmt.Sprintf("%s: %s (owner %s)", session.ErrRunBusy, e.Run.ID, e.Run.OwnerInstance)
}

func (e *BusyError) Unwrap() error { return session.ErrRunBusy }

type ClaimRequest struct {
	SessionID string
	AgentID   string
	AgentName string
	Provider  string
	Model     string
	BaseSeq   int
}

type Coordinator struct {
	sessions *session.Manager
	store    session.RunStore
	bus      event.Bus
	log      *logger.Logger
	instance string
	pid      int

	baseCtx context.Context
	cancel  context.CancelFunc

	mu     sync.Mutex
	leases map[string]*Lease
}

func New(sessions *session.Manager, bus event.Bus, log *logger.Logger) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Coordinator{
		sessions: sessions,
		bus:      bus,
		log:      log,
		instance: uuid.NewString(),
		pid:      os.Getpid(),
		baseCtx:  ctx,
		cancel:   cancel,
		leases:   make(map[string]*Lease),
	}
	if sessions != nil {
		if store, err := sessions.Runs(); err == nil {
			c.store = store
		} else if log != nil {
			log.Fdebug("run: shared run store unavailable: %v", err)
		}
	}
	return c
}

func (c *Coordinator) Instance() string { return c.instance }

func (c *Coordinator) Available() bool { return c != nil && c.store != nil }

func (c *Coordinator) Stop() {
	if c == nil {
		return
	}
	c.cancel()
	c.mu.Lock()
	leases := make([]*Lease, 0, len(c.leases))
	for _, l := range c.leases {
		leases = append(leases, l)
	}
	c.mu.Unlock()
	for _, l := range leases {
		l.Finish(session.RunStateCanceled, "process shutting down")
	}
}

func (c *Coordinator) Claim(ctx context.Context, req ClaimRequest) (*Lease, error) {
	if c == nil {
		return nil, errors.New("run: coordinator unavailable")
	}
	if req.SessionID == "" {
		return nil, errors.New("run: session id is required")
	}
	now := time.Now()
	rec := session.Run{
		ID:            uuid.NewString(),
		SessionID:     req.SessionID,
		State:         session.RunStateRunning,
		OwnerInstance: c.instance,
		OwnerPID:      c.pid,
		AgentID:       req.AgentID,
		AgentName:     req.AgentName,
		Provider:      req.Provider,
		Model:         req.Model,
		BaseSeq:       req.BaseSeq,
		StartedAt:     now,
		UpdatedAt:     now,
	}

	if c.store == nil {
		return c.newLease(ctx, rec), nil
	}

	if err := c.store.CreateRun(rec); err != nil {
		active, aerr := c.store.ActiveRun(req.SessionID)
		if aerr == nil {
			return nil, &BusyError{Run: active}
		}
		if errors.Is(err, session.ErrRunBusy) {
			return nil, &BusyError{}
		}
		return nil, fmt.Errorf("run: create run: %w", err)
	}

	if err := c.append(ctx, rec, session.RunEvent{
		Type:      session.RunEventStarted,
		AgentID:   rec.AgentID,
		AgentName: rec.AgentName,
		Data: mustJSON(map[string]any{
			"base_seq":       rec.BaseSeq,
			"owner_instance": rec.OwnerInstance,
			"owner_pid":      rec.OwnerPID,
			"provider":       rec.Provider,
			"model":          rec.Model,
			"agent_name":     rec.AgentName,
			"started_at":     rec.StartedAt.Unix(),
		}),
	}); err != nil {
		_ = c.store.FinishRun(rec.ID, session.RunStateError, err.Error())
		return nil, err
	}

	lease := c.newLease(ctx, rec)
	if c.bus != nil {
		c.bus.Publish(session.TopicRunStarted, session.RunStarted{
			RunID:     rec.ID,
			SessionID: rec.SessionID,
			AgentID:   rec.AgentID,
			AgentName: rec.AgentName,
			Provider:  rec.Provider,
			Model:     rec.Model,
			BaseSeq:   rec.BaseSeq,
		})
	}
	return lease, nil
}

func (c *Coordinator) newLease(ctx context.Context, rec session.Run) *Lease {
	if ctx == nil {
		ctx = c.baseCtx
	}
	runCtx, cancel := context.WithCancel(ctx)
	l := &Lease{
		coord:  c,
		run:    rec,
		ctx:    runCtx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	c.mu.Lock()
	c.leases[rec.ID] = l
	c.mu.Unlock()
	go c.supervise(l)
	return l
}

func (c *Coordinator) lease(id string) *Lease {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leases[id]
}

func (c *Coordinator) forget(id string) {
	c.mu.Lock()
	delete(c.leases, id)
	c.mu.Unlock()
}

func (c *Coordinator) supervise(l *Lease) {
	hb := time.NewTicker(heartbeatInterval)
	cmd := time.NewTicker(commandInterval)
	defer hb.Stop()
	defer cmd.Stop()

	for {
		select {
		case <-l.done:
			return
		case <-c.baseCtx.Done():
			l.cancel()
			return
		case <-hb.C:
			c.heartbeat(l)
		case <-cmd.C:
			c.processCommands(l)
		}
	}
}

func (c *Coordinator) heartbeat(l *Lease) {
	if c.store == nil {
		return
	}
	run := l.snapshot()
	if c.sessions != nil {
		if _, err := c.sessions.Get(run.SessionID); err != nil {
			l.cancel()
			return
		}
	}
	current, err := c.store.GetRun(run.ID)
	if err != nil {
		return
	}
	if current.State != session.RunStateRunning || current.CancelRequested {
		l.cancel()
		return
	}
	current.UpdatedAt = time.Now()
	if err := c.store.UpdateRun(*current); err != nil && c.log != nil {
		c.log.Fdebug("run: heartbeat %s: %v", run.ID, err)
	}
	l.mu.Lock()
	l.run = *current
	l.mu.Unlock()
}

func (c *Coordinator) processCommands(l *Lease) {
	if c.store == nil {
		return
	}
	run := l.snapshot()
	if current, err := c.store.GetRun(run.ID); err == nil {
		if current.State != session.RunStateRunning || current.CancelRequested {
			l.cancel()
			return
		}
	}
	cmds, err := c.store.PendingRunCommands(run.ID)
	if err != nil {
		return
	}
	for _, cmd := range cmds {
		switch cmd.Kind {
		case session.RunCommandCancel:
			l.cancel()
		case session.RunCommandAnswer:
			var payload struct {
				Answers map[string]string `json:"answers"`
			}
			if err := json.Unmarshal(cmd.Payload, &payload); err == nil {
				agentID := l.AgentID()
				if c.bus != nil && agentID != "" {
					c.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
						AgentID: agentID,
						Answers: payload.Answers,
					})
					c.bus.Publish(session.TopicRunAskResolved, session.RunAskResolved{
						RunID:     run.ID,
						SessionID: run.SessionID,
						AgentID:   agentID,
						Answers:   payload.Answers,
					})
				}
			}
		}
		if err := c.store.ConsumeRunCommand(cmd.ID); err != nil && c.log != nil {
			c.log.Fdebug("run: consume command %d: %v", cmd.ID, err)
		}
	}
}

func (c *Coordinator) append(ctx context.Context, run session.Run, e session.RunEvent) error {
	if c.store == nil {
		return nil
	}
	e.RunID = run.ID
	e.SessionID = run.SessionID
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.store.AppendRunEvents([]session.RunEvent{e})
}

func (c *Coordinator) Cancel(runID string) error {
	if c == nil {
		return errors.New("run: coordinator unavailable")
	}
	if l := c.lease(runID); l != nil {
		l.cancel()
		return nil
	}
	if c.store == nil {
		return session.ErrRunNotFound
	}
	return c.store.RequestRunCancel(runID)
}

func (c *Coordinator) CancelSession(sessionID string) error {
	if active, err := c.Active(sessionID); err == nil && active != nil {
		return c.Cancel(active.ID)
	}
	return nil
}

func (c *Coordinator) Answer(runID string, answers map[string]string) error {
	if c == nil {
		return errors.New("run: coordinator unavailable")
	}
	if l := c.lease(runID); l != nil {
		if c.bus != nil {
			c.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
				AgentID: l.AgentID(),
				Answers: answers,
			})
			c.bus.Publish(session.TopicRunAskResolved, session.RunAskResolved{
				RunID:     l.run.ID,
				SessionID: l.run.SessionID,
				AgentID:   l.AgentID(),
				Answers:   answers,
			})
		}
		return nil
	}
	if c.store == nil {
		return session.ErrRunNotFound
	}
	payload, _ := json.Marshal(map[string]any{"answers": answers})
	return c.store.AddRunCommand(session.RunCommand{
		RunID:   runID,
		Kind:    session.RunCommandAnswer,
		Payload: payload,
	})
}

func (c *Coordinator) Active(sessionID string) (*session.Run, error) {
	if c == nil || c.store == nil {
		return nil, session.ErrRunUnsupported
	}
	return c.store.ActiveRun(sessionID)
}

func (c *Coordinator) Get(runID string) (*session.Run, error) {
	if c == nil || c.store == nil {
		return nil, session.ErrRunUnsupported
	}
	return c.store.GetRun(runID)
}

func (c *Coordinator) ActiveRuns() ([]session.Run, error) {
	if c == nil || c.store == nil {
		return nil, session.ErrRunUnsupported
	}
	runs, err := c.store.ListRuns()
	if err != nil {
		return nil, err
	}
	out := make([]session.Run, 0, len(runs))
	for _, r := range runs {
		if r.State == session.RunStateRunning {
			out = append(out, r)
		}
	}
	return out, nil
}

func (c *Coordinator) IsLocal(runID string) bool {
	return c.lease(runID) != nil
}

func (c *Coordinator) ReapStale() {
	if c == nil || c.store == nil {
		return
	}
	runs, err := c.store.ListRuns()
	if err != nil {
		return
	}
	now := time.Now()
	for _, r := range runs {
		if r.State != session.RunStateRunning || r.OwnerInstance == c.instance {
			continue
		}
		if now.Sub(r.UpdatedAt) < StaleAfter {
			continue
		}
		msg := "run owner stopped responding"
		_ = c.store.AppendRunEvents([]session.RunEvent{{
			RunID:     r.ID,
			SessionID: r.SessionID,
			Type:      session.RunEventState,
			AgentID:   r.AgentID,
			Data:      mustJSON(map[string]any{"state": string(session.RunStateError), "error": msg}),
			CreatedAt: now,
		}})
		if err := c.store.FinishRun(r.ID, session.RunStateError, msg); err == nil {
			if c.log != nil {
				c.log.Fwarn("run: reaped stale run %s (session %s)", r.ID, r.SessionID)
			}
		}
	}
}

func (c *Coordinator) Watch(ctx context.Context, runID string, afterSeq int64, fn func(session.RunEvent) error) error {
	if c == nil || c.store == nil {
		return session.ErrRunUnsupported
	}
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	last := afterSeq
	finished := false
	for {
		events, err := c.store.RunEvents(runID, last, 200)
		if err != nil {
			return err
		}
		for _, e := range events {
			last = e.Seq
			if fn != nil {
				if err := fn(e); err != nil {
					return err
				}
			}
		}
		r, err := c.store.GetRun(runID)
		if err != nil {
			return err
		}
		if r.State != session.RunStateRunning {
			for !finished {
				finished = true
				events, _ := c.store.RunEvents(runID, last, 200)
				for _, e := range events {
					last = e.Seq
					if fn != nil {
						if err := fn(e); err != nil {
							return err
						}
					}
				}
				if len(events) == 200 {
					finished = false
				}
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}

type Lease struct {
	coord  *Coordinator
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	mu  sync.Mutex
	run session.Run
}

func (l *Lease) ID() string { return l.snapshot().ID }

func (l *Lease) SessionID() string { return l.snapshot().SessionID }

func (l *Lease) BaseSeq() int { return l.snapshot().BaseSeq }

func (l *Lease) snapshot() session.Run {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.run
}

func (l *Lease) AgentID() string { return l.snapshot().AgentID }

func (l *Lease) Context() context.Context { return l.ctx }

func (l *Lease) AppendEvent(e session.RunEvent) {
	if l == nil {
		return
	}
	l.mu.Lock()
	run := l.run
	l.mu.Unlock()
	if err := l.coord.append(context.Background(), run, e); err != nil && l.coord.log != nil {
		l.coord.log.Fdebug("run: append event %s: %v", e.Type, err)
	}
}

func (l *Lease) Finish(state session.RunState, errMsg string) {
	if l == nil {
		return
	}
	l.once.Do(func() {
		close(l.done)
		l.cancel()
		run := l.snapshot()

		if l.coord.bus != nil {
			l.coord.bus.Publish(session.TopicRunFinished, session.RunFinished{
				RunID:     run.ID,
				SessionID: run.SessionID,
				AgentID:   run.AgentID,
				State:     state,
				Error:     errMsg,
			})
		}
		if l.coord.store != nil {
			_ = l.coord.append(context.Background(), run, session.RunEvent{
				Type:    session.RunEventState,
				AgentID: run.AgentID,
				Data:    mustJSON(map[string]any{"state": string(state), "error": errMsg}),
			})
			_ = l.coord.store.FinishRun(run.ID, state, errMsg)
		}
		l.coord.forget(run.ID)
	})
}

func (c *Coordinator) BaseSeq(sessionID string) int {
	if c == nil || c.sessions == nil || sessionID == "" {
		return 0
	}
	msgs, err := c.sessions.Messages(sessionID)
	if err != nil {
		return 0
	}
	max := 0
	for _, m := range msgs {
		if m.Seq > max {
			max = m.Seq
		}
	}
	return max
}

func (c *Coordinator) Stream(ctx context.Context, lease *Lease, orch *agent.Agent, input string, history []llm.Message, handler agent.StreamHandler) (*agent.RunResult, error) {
	if lease == nil {
		if orch == nil {
			return nil, errors.New("run: nil agent")
		}
		if len(history) > 0 {
			return orch.ResumeStream(ctx, input, history, handler)
		}
		return orch.RunStream(ctx, input, handler)
	}
	if handler == nil {
		handler = func(agent.StreamEvent) error { return nil }
	}
	runCtx := lease.Context()
	var (
		result *agent.RunResult
		err    error
	)
	if len(history) > 0 {
		result, err = orch.ResumeStream(runCtx, input, history, handler)
	} else {
		result, err = orch.RunStream(runCtx, input, handler)
	}
	state := session.RunStateDone
	errMsg := ""
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || runCtx.Err() != nil {
			state = session.RunStateCanceled
		} else {
			state = session.RunStateError
			errMsg = err.Error()
		}
	}
	lease.Finish(state, errMsg)
	return result, err
}

func (c *Coordinator) StartMaintenance() {
	if c == nil || c.store == nil {
		return
	}
	go func() {
		stale := time.NewTicker(5 * time.Second)
		prune := time.NewTicker(24 * time.Hour)
		defer stale.Stop()
		defer prune.Stop()
		for {
			select {
			case <-c.baseCtx.Done():
				return
			case <-stale.C:
				c.ReapStale()
			case <-prune.C:
				_ = c.store.PruneRuns(time.Now().Add(-7 * 24 * time.Hour))
			}
		}
	}()
}

func (c *Coordinator) Continue(ctx context.Context, lease *Lease, orch *agent.Agent, history []llm.Message) (*agent.RunResult, error) {
	if lease == nil {
		if orch == nil {
			return nil, errors.New("run: nil agent")
		}
		return orch.Continue(ctx, history)
	}
	result, err := orch.Continue(lease.Context(), history)
	state := session.RunStateDone
	errMsg := ""
	if err != nil {
		if errors.Is(err, context.Canceled) || lease.Context().Err() != nil {
			state = session.RunStateCanceled
		} else {
			state = session.RunStateError
			errMsg = err.Error()
		}
	}
	lease.Finish(state, errMsg)
	return result, err
}

func (c *Coordinator) Events(runID string, afterSeq int64, limit int) ([]session.RunEvent, error) {
	if c == nil || c.store == nil {
		return nil, session.ErrRunUnsupported
	}
	return c.store.RunEvents(runID, afterSeq, limit)
}

func (c *Coordinator) EventsAfter(afterSeq int64, limit int) ([]session.RunEvent, error) {
	if c == nil || c.store == nil {
		return nil, session.ErrRunUnsupported
	}
	return c.store.RunEventsAfter(afterSeq, limit)
}

func (c *Coordinator) LastGlobalEventSeq() (int64, error) {
	if c == nil || c.store == nil {
		return 0, session.ErrRunUnsupported
	}
	return c.store.LastGlobalRunEventSeq()
}
