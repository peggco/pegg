package permission

import (
	"context"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
)

type promptResult struct {
	answers map[string]string
	err     error
}

func TestPromptConcurrentAgentsIsolated(t *testing.T) {
	bus := event.New()
	a := agent.New("agent-a", agent.WithBus(bus))
	b := agent.New("agent-b", agent.WithBus(bus))
	m := New(Deps{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	asked := make(chan string, 4)
	_ = bus.Subscribe(agent.TopicAgentAsk, func(e agent.AgentAsk) { asked <- e.AgentID })

	gotA := make(chan promptResult, 1)
	gotB := make(chan promptResult, 1)
	q := agent.AskQuestion{ID: "q", Question: "allow?", Type: "text", Required: true}

	go func() {
		ans, err := m.prompt(ctx, a, q)
		gotA <- promptResult{ans, err}
	}()
	go func() {
		ans, err := m.prompt(ctx, b, q)
		gotB <- promptResult{ans, err}
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-asked:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("did not see both prompts: %v", seen)
		}
	}

	bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{AgentID: a.ID, Answers: map[string]string{"q": "A"}})
	select {
	case r := <-gotA:
		if r.err != nil || r.answers["q"] != "A" {
			t.Fatalf("agent-a: %v %v", r.answers, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-a never resolved")
	}

	select {
	case r := <-gotB:
		t.Fatalf("agent-b resolved too early: %v %v", r.answers, r.err)
	case <-time.After(150 * time.Millisecond):
	}

	bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{AgentID: b.ID, Answers: map[string]string{"q": "B"}})
	select {
	case r := <-gotB:
		if r.err != nil || r.answers["q"] != "B" {
			t.Fatalf("agent-b: %v %v", r.answers, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-b never resolved")
	}
}
