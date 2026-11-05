package acp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/live"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
)

func newSessionsChangedServer(t *testing.T) (*Server, *bufferTransport) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := session.NewSQLiteStoreAt(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	bus := event.New()
	log := logger.New(logger.LevelError)
	mgr := session.NewManager(store, bus, log)
	coord := runpkg.New(mgr, bus, log)
	t.Cleanup(coord.Stop)
	hub := live.New(coord, mgr, bus, log)
	t.Cleanup(hub.Close)

	srv := New(config.DefaultConfig(), bus, log, nil, mgr, nil, coord, hub)
	tr := newBufferTransport()
	srv.transport = tr
	return srv, tr
}

func TestSessionsChangedNotification(t *testing.T) {
	srv, tr := newSessionsChangedServer(t)

	srv.handleSessionCreated(session.SessionCreated{SessionID: "s1"})
	srv.handleSessionUpdated(session.SessionUpdated{SessionID: "s1"})
	srv.handleSessionDeleted(session.SessionDeleted{SessionID: "s1"})

	tr.mu.Lock()
	sends := append([]string(nil), tr.sends...)
	tr.mu.Unlock()
	count := 0
	for _, raw := range sends {
		if strings.Contains(raw, "pegg/sessions_changed") {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("sessions_changed notifications = %d, want 3", count)
	}
}
