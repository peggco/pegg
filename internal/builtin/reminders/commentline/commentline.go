package commentline

import (
	"fmt"
	"sync"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/reminder"
	"github.com/peggco/pegg/internal/core/event"
)

const (
	writeTool = "write"
	editTool  = "edit"
	everyN    = 20
)

const reminderContent = `Friendly reminder: you just added code comments. Unless the user explicitly asked for comments, do not add them — prefer clear, self-explanatory code and keep comments out. Ignore this reminder if the user did ask for comments.`

type agentState struct {
	agent  *agent.Agent
	total  int
	fired  bool
	lastAt int
}

type Reminder struct {
	mu     sync.Mutex
	states map[string]*agentState
}

func New() *Reminder {
	return &Reminder{states: make(map[string]*agentState)}
}

func (r *Reminder) Start(bus event.Bus) error {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.handleStarted},
		{agent.TopicAgentToolCall, r.handleToolCall},
		{agent.TopicAgentFinished, r.handleFinished},
		{agent.TopicAgentError, r.handleError},
	}
	for _, s := range subs {
		if err := bus.Subscribe(s.topic, s.fn); err != nil {
			return fmt.Errorf("commentline reminder: subscribe %s: %w", s.topic, err)
		}
	}
	return nil
}

func (r *Reminder) Stop(bus event.Bus) error {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.handleStarted},
		{agent.TopicAgentToolCall, r.handleToolCall},
		{agent.TopicAgentFinished, r.handleFinished},
		{agent.TopicAgentError, r.handleError},
	}
	var firstErr error
	for _, s := range subs {
		if err := bus.Unsubscribe(s.topic, s.fn); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("commentline reminder: unsubscribe %s: %w", s.topic, err)
		}
	}
	return firstErr
}

func (r *Reminder) handleStarted(e agent.AgentStarted) {
	if e.Agent == nil {
		return
	}
	r.mu.Lock()
	s, ok := r.states[e.AgentID]
	if !ok {
		s = &agentState{}
		r.states[e.AgentID] = s
	}
	s.agent = e.Agent
	r.mu.Unlock()
}

func (r *Reminder) handleToolCall(e agent.AgentToolCall) {
	var added int
	switch e.Call.Function.Name {
	case writeTool:
		added = addedCommentsFromWrite(e.Call.Function.Arguments)
	case editTool:
		added = addedCommentsFromEdit(e.Call.Function.Arguments)
	default:
		return
	}
	if added <= 0 {
		return
	}
	r.bump(e.AgentID, added)
}

func (r *Reminder) bump(agentID string, added int) {
	r.mu.Lock()
	s, ok := r.states[agentID]
	if !ok || s.agent == nil {
		r.mu.Unlock()
		return
	}
	fire := scheduleFire(s, added)
	a := s.agent
	r.mu.Unlock()

	if fire {
		a.QueueDisplayNotification(reminder.New("comment", reminderContent))
	}
}

func scheduleFire(s *agentState, added int) bool {
	s.total += added
	switch {
	case !s.fired && s.total >= 1:
		s.fired = true
		s.lastAt = s.total
		return true
	case s.fired && s.total-s.lastAt >= everyN:
		s.lastAt = s.total
		return true
	}
	return false
}

func (r *Reminder) handleFinished(e agent.AgentFinished) { r.cleanup(e.AgentID) }
func (r *Reminder) handleError(e agent.AgentError)       { r.cleanup(e.AgentID) }

func (r *Reminder) cleanup(agentID string) {
	r.mu.Lock()
	delete(r.states, agentID)
	r.mu.Unlock()
}

func addedCommentsFromWrite(args string) int {
	var p struct {
		FilePath string `json:"filePath"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return 0
	}
	return countCommentLines(p.Content, styleForPath(p.FilePath))
}

func addedCommentsFromEdit(args string) int {
	var p struct {
		FilePath  string `json:"filePath"`
		OldString string `json:"oldString"`
		NewString string `json:"newString"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return 0
	}
	st := styleForPath(p.FilePath)
	added := countCommentLines(p.NewString, st) - countCommentLines(p.OldString, st)
	if added < 0 {
		added = 0
	}
	return added
}
