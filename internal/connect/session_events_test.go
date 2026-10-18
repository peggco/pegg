package connect

import (
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/session"
)

func TestSessionEventsForwardedToClient(t *testing.T) {
	bus := event.New()
	c, err := New(Deps{Config: config.DefaultConfig(), Bus: bus, Log: testLogger(), Version: "test"}, config.DefaultConfig().Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	cs := &connState{out: make(chan *Frame, 16), done: make(chan struct{})}
	c.cur.Store(cs)

	cases := []struct {
		name    string
		publish func()
	}{
		{"updated", func() { bus.Publish(session.TopicSessionUpdated, session.SessionUpdated{SessionID: "s1"}) }},
		{"created", func() { bus.Publish(session.TopicSessionCreated, session.SessionCreated{SessionID: "s2"}) }},
		{"deleted", func() { bus.Publish(session.TopicSessionDeleted, session.SessionDeleted{SessionID: "s3"}) }},
	}
	wantSessions := []string{"s1", "s2", "s3"}

	for i, tc := range cases {
		tc.publish()
		select {
		case f := <-cs.out:
			if f.Type != frameEvent || f.Event != eventSessionUpdate {
				t.Fatalf("%s: got type=%s event=%s, want event=%s", tc.name, f.Type, f.Event, eventSessionUpdate)
			}
			if f.SessionID != wantSessions[i] {
				t.Fatalf("%s: session id = %q, want %q", tc.name, f.SessionID, wantSessions[i])
			}
		case <-time.After(time.Second):
			t.Fatalf("%s: timed out waiting for %s event", tc.name, eventSessionUpdate)
		}
	}
}
