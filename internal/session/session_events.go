package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/peggco/pegg/internal/core/event"
)

type SessionEvent struct {
	Seq       int64
	Type      string
	SessionID string
	Data      json.RawMessage
	Origin    string
	CreatedAt time.Time
}

const (
	SessionEventCreated = "created"
	SessionEventUpdated = "updated"
	SessionEventDeleted = "deleted"
)

type SessionEventStore interface {
	AppendSessionEvent(ev SessionEvent) error
	SessionEvents(afterSeq int64, limit int) ([]SessionEvent, error)
	LastSessionEventSeq() (int64, error)
}

func (m *Manager) persistChange(kind, sessionID string, data any) {
	store, ok := m.store.(SessionEventStore)
	if !ok || sessionID == "" {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		payload = []byte("{}")
	}
	_ = store.AppendSessionEvent(SessionEvent{
		Type:      kind,
		SessionID: sessionID,
		Data:      payload,
		Origin:    m.origin,
		CreatedAt: time.Now(),
	})
}

func (m *Manager) StartEventWatcher(bus event.Bus) func() {
	store, ok := m.store.(SessionEventStore)
	if !ok || bus == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		last, _ := store.LastSessionEventSeq()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := store.SessionEvents(last, 200)
				if err != nil {
					continue
				}
				for _, e := range events {
					last = e.Seq
					if e.Origin == m.origin {
						continue
					}
					switch e.Type {
					case SessionEventCreated:
						bus.Publish(TopicSessionCreated, SessionCreated{SessionID: e.SessionID})
					case SessionEventUpdated:
						bus.Publish(TopicSessionUpdated, SessionUpdated{SessionID: e.SessionID})
					case SessionEventDeleted:
						bus.Publish(TopicSessionDeleted, SessionDeleted{SessionID: e.SessionID})
					}
				}
			}
		}
	}()
	return cancel
}
