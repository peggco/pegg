package todo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

type Todo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DependsOn   []string  `json:"dependsOn,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

const defaultSessionID = "default"

type sessionStore struct {
	file   string
	todos  []*Todo
	loaded bool
}

type todoStore struct {
	mu            sync.Mutex
	bus           event.Bus
	agentSessions map[string]string
	sessions      map[string]*sessionStore
}

var store = &todoStore{
	agentSessions: make(map[string]string),
	sessions:      make(map[string]*sessionStore),
}

func TodoTools(fs *vfs.VFS, bus event.Bus) {
	store.subscribe(bus)
	tools.Register(todoreadTool(fs))
	tools.Register(todowriteTool(fs))
}

func (s *todoStore) subscribe(bus event.Bus) {
	if bus == nil {
		return
	}
	s.mu.Lock()
	if s.bus == bus {
		s.mu.Unlock()
		return
	}
	s.bus = bus
	s.mu.Unlock()
	_ = bus.Subscribe(session.TopicSessionAttached, func(attached session.SessionAttached) {
		s.onSessionAttached(attached.AgentID, attached.SessionID)
	})
}

func (s *todoStore) onSessionAttached(agentID, sessionID string) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentSessions[agentID] = sessionID
}

func (s *todoStore) sessionIDLocked(ctx context.Context) string {
	if a := agent.FromContext(ctx); a != nil {
		if id := s.agentSessions[a.ID]; id != "" {
			return id
		}
	}
	return defaultSessionID
}

func (s *todoStore) initFor(sessionID string) (*sessionStore, error) {
	ss, ok := s.sessions[sessionID]
	if !ok {
		ss = &sessionStore{}
		s.sessions[sessionID] = ss
	}
	if ss.loaded {
		return ss, nil
	}
	if err := config.EnsureProjectConfigDir(); err != nil {
		return nil, fmt.Errorf("todo: ensure config dir: %w", err)
	}
	file, err := config.GetProjectConfigPath("todos", sessionID+".json")
	if err != nil {
		return nil, fmt.Errorf("todo: config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, fmt.Errorf("todo: ensure todos dir: %w", err)
	}
	ss.file = file

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			ss.loaded = true
			return ss, nil
		}
		return nil, fmt.Errorf("todo: read file: %w", err)
	}
	var list []*Todo
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("todo: parse: %w", err)
	}
	ss.todos = list
	ss.loaded = true
	return ss, nil
}

func (s *todoStore) save(ss *sessionStore) error {
	data, err := json.MarshalIndent(ss.todos, "", "  ")
	if err != nil {
		return fmt.Errorf("todo: marshal: %w", err)
	}
	if err := os.WriteFile(ss.file, data, 0o644); err != nil {
		return fmt.Errorf("todo: write: %w", err)
	}
	return nil
}

func (s *todoStore) all(ctx context.Context) ([]*Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, err := s.initFor(s.sessionIDLocked(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]*Todo, len(ss.todos))
	copy(out, ss.todos)
	return out, nil
}

func (s *todoStore) setAll(ctx context.Context, todos []*Todo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, err := s.initFor(s.sessionIDLocked(ctx))
	if err != nil {
		return err
	}
	now := time.Now()
	next := make([]*Todo, 0, len(todos))
	index := make(map[string]int, len(todos))
	for _, t := range todos {
		if t.CreatedAt.IsZero() {
			t.CreatedAt = now
		}
		t.UpdatedAt = now
		if i, ok := index[t.ID]; ok {
			next[i] = t
			continue
		}
		index[t.ID] = len(next)
		next = append(next, t)
	}
	ss.todos = next
	return s.save(ss)
}

func formatTodoList(todos []*Todo) string {
	if len(todos) == 0 {
		return "No todos.\n"
	}
	var b strings.Builder
	for _, t := range todos {
		status := statusIcon(t.Status)
		priority := priorityLabel(t.Priority)
		fmt.Fprintf(&b, "%s [%s] %s (priority: %s)\n", status, t.ID, t.Title, priority)
		if t.Description != "" {
			fmt.Fprintf(&b, "   %s\n", t.Description)
		}
		if len(t.DependsOn) > 0 {
			fmt.Fprintf(&b, "   depends on: %s\n", strings.Join(t.DependsOn, ", "))
		}
	}
	return b.String()
}

func statusIcon(status string) string {
	switch status {
	case "completed":
		return "[x]"
	case "in_progress":
		return "[~]"
	case "cancelled":
		return "[-]"
	default:
		return "[ ]"
	}
}

func priorityLabel(p string) string {
	switch p {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
