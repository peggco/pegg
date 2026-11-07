package session

import (
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
)

func TestRecorderTitlesPrecreatedSession(t *testing.T) {
	mgr, _, bus := newTestRecorder(t)
	prov := &titleCountingProvider{}

	sess, err := mgr.Create(CreateOptions{Title: "ACP Session", Provider: "title", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	bus.Publish(TopicSessionResume, SessionResume{AgentID: "a1", SessionID: sess.ID})
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "a1", AgentName: "orchestrator",
		Model:    llm.Model{ID: "m"},
		Provider: prov,
	})
	bus.Publish(agent.TopicAgentInput, agent.AgentInput{AgentID: "a1", Input: "fix the auth bug"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := mgr.Get(sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title == "Session title" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := mgr.Get(sess.ID)
	t.Fatalf("title = %q, want generated title (provider calls=%d)", got.Title, prov.Count())
}
