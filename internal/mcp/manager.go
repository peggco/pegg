package mcp

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

type connectHandle struct {
	cancel context.CancelFunc
	seq    uint64
}

type Manager struct {
	mu         sync.Mutex
	clients    map[string]*Client
	configs    map[string]config.MCPServerConfig
	connecting map[string]connectHandle
	log        *logger.Logger
	wg         sync.WaitGroup
	pending    atomic.Int64
	seq        atomic.Uint64
	closed     bool
	version    atomic.Uint64
}

func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		configs:    make(map[string]config.MCPServerConfig),
		connecting: make(map[string]connectHandle),
		log:        log,
	}
}

func (m *Manager) Start(servers map[string]config.MCPServerConfig) {
	m.Reload(servers)
}

func (m *Manager) Reload(servers map[string]config.MCPServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}

	for name, cfg := range m.configs {
		next, ok := servers[name]
		if ok && reflect.DeepEqual(next, cfg) {
			continue
		}
		m.stopLocked(name)
	}

	for name, cfg := range servers {
		if _, ok := m.clients[name]; ok {
			continue
		}
		if _, ok := m.connecting[name]; ok {
			continue
		}
		m.startLocked(name, cfg)
	}
}

func (m *Manager) startLocked(name string, cfg config.MCPServerConfig) {
	ctx, cancel := context.WithCancel(context.Background())
	seq := m.seq.Add(1)
	m.configs[name] = cfg
	m.connecting[name] = connectHandle{cancel: cancel, seq: seq}
	m.pending.Add(1)
	m.wg.Add(1)
	go m.connect(name, cfg, ctx, seq)
}

func (m *Manager) stopLocked(name string) {
	if handle, ok := m.connecting[name]; ok {
		handle.cancel()
		delete(m.connecting, name)
	}
	if client, ok := m.clients[name]; ok {
		_ = client.Close()
		delete(m.clients, name)
	}
	delete(m.configs, name)
	m.unregisterServerTools(name)
	m.version.Add(1)
}

func (m *Manager) clearConnecting(name string, seq uint64) {
	m.mu.Lock()
	if handle, ok := m.connecting[name]; ok && handle.seq == seq {
		handle.cancel()
		delete(m.connecting, name)
	}
	m.mu.Unlock()
}

func (m *Manager) connect(name string, cfg config.MCPServerConfig, parent context.Context, seq uint64) {
	defer m.wg.Done()
	defer m.pending.Add(-1)
	defer m.version.Add(1)
	defer m.clearConnecting(name, seq)

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	kind, err := ResolveTransportKind(cfg)
	if err != nil {
		m.connectError(name, parent, "mcp: server %q: %v", err)
		return
	}
	m.log.Fdebug("mcp: server %q transport %s", name, kind)

	transport, err := NewTransport(cfg, m.log.Writer(logger.LevelDebug, "mcp["+name+"]: "))
	if err != nil {
		m.connectError(name, parent, "mcp: server %q: %v", err)
		return
	}

	if err := transport.Connect(ctx); err != nil {
		_ = transport.Close()
		m.connectError(name, parent, "mcp: server %q: connect: %v", err)
		return
	}

	client := NewClient(name, transport, Options{Name: name, ProtocolVersion: kind.ProtocolVersion()})
	if _, err := client.Initialize(ctx); err != nil {
		_ = client.Close()
		m.connectError(name, parent, "mcp: server %q: initialize: %v", err)
		return
	}

	specs, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Close()
		m.connectError(name, parent, "mcp: server %q: list tools: %v", err)
		return
	}

	m.mu.Lock()
	if m.closed || parent.Err() != nil {
		m.mu.Unlock()
		_ = client.Close()
		if !m.closed {
			m.log.Fdebug("mcp: server %q: connect superseded", name)
		}
		return
	}
	for _, spec := range specs {
		t := NewTool(name, spec, client)
		if err := tools.Register(t); err != nil {
			m.log.Fdebug("mcp: server %q: register tool %q: %v", name, t.Name(), err)
			continue
		}
	}
	m.clients[name] = client
	m.mu.Unlock()

	m.log.Finfo("mcp: server %q connected (%d tools)", name, len(specs))
}

func (m *Manager) connectError(name string, parent context.Context, format string, err error) {
	if errors.Is(parent.Err(), context.Canceled) {
		m.log.Fdebug("mcp: server %q: canceled", name)
		return
	}
	m.log.Fwarn(format, name, err)
}

func (m *Manager) unregisterServerTools(server string) {
	for _, t := range tools.List() {
		mt, ok := t.(*MCPTool)
		if !ok || mt.Server() != server {
			continue
		}
		tools.Unregister(t.Name())
	}
}

func (m *Manager) Clients() map[string]*Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]*Client, len(m.clients))
	for name, c := range m.clients {
		out[name] = c
	}
	return out
}

func (m *Manager) Pending() int64 {
	return m.pending.Load()
}

func (m *Manager) Ready() bool {
	return m.pending.Load() == 0
}

func (m *Manager) WaitReady(ctx context.Context) error {
	if m.Ready() {
		return nil
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.Ready() {
				return nil
			}
		}
	}
}

func (m *Manager) Version() uint64 {
	return m.version.Load()
}

func (m *Manager) Wait() {
	m.wg.Wait()
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	for name, handle := range m.connecting {
		handle.cancel()
		delete(m.connecting, name)
	}
	names := make([]string, 0, len(m.clients))
	for name, c := range m.clients {
		names = append(names, name)
		_ = c.Close()
	}
	m.clients = make(map[string]*Client)
	m.configs = make(map[string]config.MCPServerConfig)
	m.mu.Unlock()

	tools.SetDynamicReady(nil)
	for _, name := range names {
		m.unregisterServerTools(name)
	}

	m.wg.Wait()
	m.log.Debug("mcp: all servers closed")
	return nil
}

func NewTransport(cfg config.MCPServerConfig, stderr io.Writer) (Duplex, error) {
	kind, err := ResolveTransportKind(cfg)
	if err != nil {
		return nil, err
	}
	switch kind {
	case TransportStreamableHTTP:
		return NewStreamableHTTPTransport(StreamableHTTPOptions{
			URL:             cfg.URL,
			Headers:         cfg.Headers,
			ProtocolVersion: kind.ProtocolVersion(),
		})
	case TransportSSE:
		return NewSSETransport(SSEOptions{
			URL:     cfg.URL,
			Headers: cfg.Headers,
		})
	case TransportStdio:
		return NewStdioTransport(StdioOptions{
			Command: cfg.Command,
			Args:    cfg.Args,
			Env:     cfg.Env,
			Stderr:  stderr,
		})
	default:
		return nil, ErrUnsupported
	}
}
