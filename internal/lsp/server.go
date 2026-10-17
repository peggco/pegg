package lsp

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/peggco/pegg/internal/core/config"
)

type docState struct {
	opened  bool
	version int
}

type Server struct {
	name string
	cfg  config.LanguageServerConfig

	mu       sync.Mutex
	client   *Client
	started  bool
	failed   bool
	starting bool
	docs     map[string]*docState
	pending  map[string]bool
	lastUsed time.Time
}

func NewServer(name string, cfg config.LanguageServerConfig) *Server {
	return &Server{
		name:     name,
		cfg:      cfg,
		docs:     make(map[string]*docState),
		pending:  make(map[string]bool),
		lastUsed: time.Now(),
	}
}

func (s *Server) Touch() {
	s.mu.Lock()
	s.lastUsed = time.Now()
	s.mu.Unlock()
}

func (s *Server) IdleFor() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastUsed)
}

func (s *Server) Matches(path string) bool {
	ext := extensionOf(path)
	for _, ft := range s.cfg.FileTypes {
		if ft == ext {
			return true
		}
	}
	return false
}

func (s *Server) Client() *Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client != nil && s.started
}

func (s *Server) IsFailed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failed
}

func (s *Server) openDoc(item TextDocumentItem) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.openLocked(item); err != nil {
		return 0, err
	}
	s.lastUsed = time.Now()
	return s.docs[item.URI].version, nil
}

func (s *Server) syncDocs(item TextDocumentItem) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ds, err := s.openLocked(item)
	if err != nil {
		return 0, err
	}
	ds.version++
	version := ds.version
	if err := s.client.DidChange(context.Background(), item.URI, version, item.Text); err != nil {
		return 0, err
	}
	if err := s.client.DidSave(context.Background(), item.URI, item.Text); err != nil {
		return 0, err
	}
	s.lastUsed = time.Now()
	return version, nil
}

func (s *Server) openLocked(item TextDocumentItem) (*docState, error) {
	if s.client == nil || !s.started {
		return nil, ErrNotRunning
	}
	if ds := s.docs[item.URI]; ds != nil && ds.opened {
		return ds, nil
	}
	open := item
	open.Version = 1
	if err := s.client.DidOpen(context.Background(), open); err != nil {
		return nil, err
	}
	ds := &docState{opened: true, version: 1}
	s.docs[item.URI] = ds
	return ds, nil
}

func (s *Server) openURIs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.docs))
	for uri := range s.docs {
		out = append(out, uri)
	}
	return out
}

func (s *Server) stop() {
	s.mu.Lock()
	c := s.client
	s.mu.Unlock()
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Shutdown(ctx)
	_ = c.Close()

	s.mu.Lock()
	s.client = nil
	s.started = false
	s.docs = make(map[string]*docState)
	s.pending = make(map[string]bool)
	s.mu.Unlock()
}

func extensionOf(path string) string {
	i := strings.LastIndexByte(path, '.')
	if i < 0 || i == len(path)-1 {
		return ""
	}
	return path[i+1:]
}
