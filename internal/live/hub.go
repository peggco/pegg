package live

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

const (
	pollInterval       = 250 * time.Millisecond
	activePollInterval = 75 * time.Millisecond
	pageSize           = 500
	queueLimit         = 20000
)

type Hub struct {
	coord    *run.Coordinator
	sessions *session.Manager
	bus      event.Bus
	log      *logger.Logger

	instance string
	ctx      context.Context
	cancel   context.CancelFunc

	stopSessionWatch func()

	mu     sync.Mutex
	last   int64
	nextID int64
	subs   map[int64]*subscriber
	runs   map[string]session.Run
	active map[string]bool
	closed bool
}

func New(coord *run.Coordinator, sessions *session.Manager, bus event.Bus, log *logger.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		coord:    coord,
		sessions: sessions,
		bus:      bus,
		log:      log,
		instance: "",
		ctx:      ctx,
		cancel:   cancel,
		subs:     make(map[int64]*subscriber),
		runs:     make(map[string]session.Run),
		active:   make(map[string]bool),
	}
	if coord != nil {
		h.instance = coord.Instance()
	}
	if coord != nil {
		if seq, err := coord.LastGlobalEventSeq(); err == nil {
			h.last = seq
		}
	}
	if sessions != nil {
		h.stopSessionWatch = sessions.StartEventWatcher(bus)
	}
	go h.loop()
	return h
}

func (h *Hub) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	subs := make([]*subscriber, 0, len(h.subs))
	for _, s := range h.subs {
		subs = append(subs, s)
	}
	h.subs = make(map[int64]*subscriber)
	h.mu.Unlock()
	h.cancel()
	if h.stopSessionWatch != nil {
		h.stopSessionWatch()
	}
	for _, s := range subs {
		s.close()
	}
}

func (h *Hub) loop() {
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-timer.C:
			h.poll()
			timer.Reset(h.interval())
		}
	}
}

func (h *Hub) interval() time.Duration {
	h.mu.Lock()
	active := len(h.subs) > 0 && len(h.active) > 0
	h.mu.Unlock()
	if active {
		return activePollInterval
	}
	return pollInterval
}

func (h *Hub) poll() {
	if h == nil || h.coord == nil || !h.hasSubscribers() {
		return
	}
	for {
		events, err := h.coord.EventsAfter(h.currentSeq(), pageSize)
		if err != nil {
			return
		}
		for _, e := range events {
			h.mu.Lock()
			h.last = e.Seq
			h.mu.Unlock()
			h.dispatch(e)
		}
		if len(events) < pageSize {
			return
		}
	}
}

func (h *Hub) currentSeq() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.last
}

func (h *Hub) hasSubscribers() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

func (h *Hub) dispatch(e session.RunEvent) {
	runInfo, ok := h.runFor(e)
	local := ok && runInfo.OwnerInstance != "" && runInfo.OwnerInstance == h.instance
	if h.coord != nil {
		local = h.coord.IsLocal(e.RunID)
	}

	out := Event{
		Type:    EventRunEvent,
		Run:     runInfo,
		Seq:     e.Seq,
		Journal: e,
		Local:   local,
	}
	switch e.Type {
	case session.RunEventStarted:
		out.Type = EventRunStarted
		h.mu.Lock()
		h.active[e.RunID] = true
		h.mu.Unlock()
	case session.RunEventState:
		out.Type = EventRunFinished
		out.State, out.Error, _ = DecodeRunState(e)
		h.mu.Lock()
		delete(h.active, e.RunID)
		h.mu.Unlock()
		h.forgetRun(e.RunID)
	case session.RunEventAskResolved:
		// Keep active; this is not a terminal event.
	default:
		h.mu.Lock()
		h.active[e.RunID] = true
		h.mu.Unlock()
	}
	h.broadcast(out)
}

func (h *Hub) runFor(e session.RunEvent) (session.Run, bool) {
	h.mu.Lock()
	r, ok := h.runs[e.RunID]
	h.mu.Unlock()
	if ok {
		return r, true
	}
	if h.coord != nil {
		if got, err := h.coord.Get(e.RunID); err == nil && got != nil {
			h.mu.Lock()
			h.runs[e.RunID] = *got
			h.mu.Unlock()
			return *got, true
		}
	}
	r = session.Run{
		ID:        e.RunID,
		SessionID: e.SessionID,
		AgentID:   e.AgentID,
		AgentName: e.AgentName,
		State:     session.RunStateRunning,
	}
	if e.Type == session.RunEventStarted {
		var data struct {
			BaseSeq       int    `json:"base_seq"`
			OwnerInstance string `json:"owner_instance"`
			Provider      string `json:"provider"`
			Model         string `json:"model"`
			AgentName     string `json:"agent_name"`
		}
		if json.Unmarshal(e.Data, &data) == nil {
			r.BaseSeq = data.BaseSeq
			r.OwnerInstance = data.OwnerInstance
			r.Provider = data.Provider
			r.Model = data.Model
			if data.AgentName != "" {
				r.AgentName = data.AgentName
			}
		}
		h.mu.Lock()
		h.runs[e.RunID] = r
		h.mu.Unlock()
	}
	return r, false
}

func (h *Hub) forgetRun(id string) {
	h.mu.Lock()
	delete(h.runs, id)
	h.mu.Unlock()
}

func (h *Hub) broadcast(e Event) {
	h.mu.Lock()
	subs := make([]*subscriber, 0, len(h.subs))
	for _, s := range h.subs {
		subs = append(subs, s)
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.push(e)
	}
}

func (h *Hub) SnapshotSession(sessionID string, includeLocal bool) ([]Event, int64, error) {
	if h == nil || h.coord == nil {
		return nil, 0, session.ErrRunUnsupported
	}
	since, err := h.coord.LastGlobalEventSeq()
	if err != nil {
		return nil, 0, err
	}
	r, err := h.coord.Active(sessionID)
	if err != nil || r == nil {
		return nil, since, nil
	}
	if !includeLocal && h.coord.IsLocal(r.ID) {
		return nil, since, nil
	}
	events, err := h.replay(*r, since, includeLocal)
	if err != nil {
		return nil, since, err
	}
	return events, since, nil
}

func (h *Hub) SnapshotAll(includeLocal bool) ([]Event, int64, error) {
	if h == nil || h.coord == nil {
		return nil, 0, session.ErrRunUnsupported
	}
	since, err := h.coord.LastGlobalEventSeq()
	if err != nil {
		return nil, 0, err
	}
	runs, err := h.coord.ActiveRuns()
	if err != nil {
		return nil, since, err
	}
	var out []Event
	for i := range runs {
		r := runs[i]
		if !includeLocal && h.coord.IsLocal(r.ID) {
			continue
		}
		events, rerr := h.replay(r, since, includeLocal)
		if rerr != nil {
			return nil, since, rerr
		}
		out = append(out, events...)
	}
	return out, since, nil
}

func (h *Hub) replay(r session.Run, max int64, includeLocal bool) ([]Event, error) {
	local := h.coord.IsLocal(r.ID)
	out := []Event{{
		Type:  EventRunStarted,
		Run:   r,
		Local: local,
	}}
	var after int64
	for {
		events, err := h.coord.Events(r.ID, after, pageSize)
		if err != nil {
			return out, err
		}
		for _, e := range events {
			after = e.Seq
			if e.Seq > max {
				continue
			}
			if e.Type == session.RunEventStarted {
				continue
			}
			ev := Event{Type: EventRunEvent, Run: r, Seq: e.Seq, Journal: e, Local: local}
			if e.Type == session.RunEventState {
				ev.Type = EventRunFinished
				ev.State, ev.Error, _ = DecodeRunState(e)
			}
			out = append(out, ev)
		}
		if len(events) < pageSize {
			break
		}
	}
	return out, nil
}

func (h *Hub) SubscribeSession(sessionID string, includeLocal bool, since int64) *Subscription {
	return h.subscribe(sessionID, includeLocal, since)
}

func (h *Hub) SubscribeAll(includeLocal bool, since int64) *Subscription {
	return h.subscribe("", includeLocal, since)
}

func (h *Hub) subscribe(sessionID string, includeLocal bool, since int64) *Subscription {
	if h == nil || h.coord == nil {
		return &Subscription{C: closedChannel()}
	}
	if since <= 0 {
		if seq, err := h.coord.LastGlobalEventSeq(); err == nil {
			since = seq
		}
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return &Subscription{C: closedChannel()}
	}
	if len(h.subs) == 0 && since > h.last {
		h.last = since
	}
	h.nextID++
	id := h.nextID
	s := &subscriber{
		id:           id,
		hub:          h,
		sessionID:    sessionID,
		includeLocal: includeLocal,
		since:        since,
		out:          make(chan Event, 256),
		signal:       make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
	h.subs[id] = s
	h.mu.Unlock()
	go s.run()
	return &Subscription{C: s.out, sub: s}
}

func closedChannel() <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

type Subscription struct {
	C   <-chan Event
	sub *subscriber
}

func (s *Subscription) Close() {
	if s != nil && s.sub != nil {
		s.sub.close()
	}
}

type subscriber struct {
	id           int64
	hub          *Hub
	sessionID    string
	includeLocal bool

	mu    sync.Mutex
	since int64
	queue []Event

	out      chan Event
	signal   chan struct{}
	done     chan struct{}
	closeOne sync.Once
}

func (s *subscriber) matches(e Event) bool {
	if s.sessionID != "" && e.Journal.SessionID != s.sessionID && e.Run.SessionID != s.sessionID {
		return false
	}
	if !s.includeLocal && e.Local {
		return false
	}
	return true
}

func (s *subscriber) push(e Event) {
	s.mu.Lock()
	if e.Seq > 0 && e.Seq <= s.since {
		s.mu.Unlock()
		return
	}
	if e.Seq > 0 {
		s.since = e.Seq
	}
	ok := s.matches(e)
	if !ok {
		s.mu.Unlock()
		return
	}
	if len(s.queue) >= queueLimit {
		s.queue = s.queue[1:]
	}
	s.queue = append(s.queue, e)
	s.mu.Unlock()
	select {
	case s.signal <- struct{}{}:
	default:
	}
}

func (s *subscriber) run() {
	defer close(s.out)
	for {
		select {
		case <-s.done:
			return
		case <-s.signal:
			for {
				s.mu.Lock()
				if len(s.queue) == 0 {
					s.mu.Unlock()
					break
				}
				e := s.queue[0]
				s.queue = s.queue[1:]
				s.mu.Unlock()
				select {
				case s.out <- e:
				case <-s.done:
					return
				}
			}
		}
	}
}

func (s *subscriber) close() {
	if s == nil {
		return
	}
	s.closeOne.Do(func() {
		close(s.done)
		s.hub.mu.Lock()
		delete(s.hub.subs, s.id)
		s.hub.mu.Unlock()
	})
}
