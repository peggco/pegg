package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/event"
)

type askResult struct {
	answers map[string]string
	err     error
}

func TestAskConcurrentWaitersIsolated(t *testing.T) {
	bus := event.New()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	asked := make(chan string, 4)
	_ = bus.Subscribe(TopicAgentAsk, func(e AgentAsk) { asked <- e.AgentID })

	gotA := make(chan askResult, 1)
	gotB := make(chan askResult, 1)

	go func() {
		ans, err := Ask(ctx, bus, "agent-a", "req-a", func() {
			bus.Publish(TopicAgentAsk, AgentAsk{AgentID: "agent-a", RequestID: "req-a"})
		})
		gotA <- askResult{ans, err}
	}()
	go func() {
		ans, err := Ask(ctx, bus, "agent-b", "req-b", func() {
			bus.Publish(TopicAgentAsk, AgentAsk{AgentID: "agent-b", RequestID: "req-b"})
		})
		gotB <- askResult{ans, err}
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-asked:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("did not see both asks: %v", seen)
		}
	}

	bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{AgentID: "agent-a", RequestID: "req-a", Answers: map[string]string{"q": "A"}})

	select {
	case r := <-gotA:
		if r.err != nil || r.answers["q"] != "A" {
			t.Fatalf("agent-a: answers=%v err=%v", r.answers, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-a never resolved")
	}

	select {
	case r := <-gotB:
		t.Fatalf("agent-b resolved too early: answers=%v err=%v", r.answers, r.err)
	case <-time.After(150 * time.Millisecond):
	}

	bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{AgentID: "agent-b", RequestID: "req-b", Answers: map[string]string{"q": "B"}})

	select {
	case r := <-gotB:
		if r.err != nil || r.answers["q"] != "B" {
			t.Fatalf("agent-b: answers=%v err=%v", r.answers, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-b never resolved")
	}
}

func TestAskSynchronousAnswerDuringPublish(t *testing.T) {
	bus := event.New()
	_ = bus.Subscribe(TopicAgentAsk, func(e AgentAsk) {
		bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{
			AgentID:   e.AgentID,
			RequestID: e.RequestID,
			Answers:   map[string]string{"q": "ok"},
		})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ans, err := Ask(ctx, bus, "solo", "rid", func() {
		bus.Publish(TopicAgentAsk, AgentAsk{AgentID: "solo", RequestID: "rid"})
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ans["q"] != "ok" {
		t.Fatalf("answers = %v", ans)
	}
}

func TestAskWildcardRequestID(t *testing.T) {
	bus := event.New()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{AgentID: "solo", Answers: map[string]string{"q": "v"}})
	}()

	ans, err := Ask(ctx, bus, "solo", "rid", func() {
		bus.Publish(TopicAgentAsk, AgentAsk{AgentID: "solo", RequestID: "rid"})
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ans["q"] != "v" {
		t.Fatalf("answers = %v", ans)
	}
}

func TestAskDirectConcurrentPublish(t *testing.T) {
	bus := event.New()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan string, 2)
	for _, id := range []string{"x", "y"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			ans, err := Ask(ctx, bus, id, "req-"+id, func() {
				bus.Publish(TopicAgentAsk, AgentAsk{AgentID: id, RequestID: "req-" + id})
			})
			if err != nil {
				results <- "err:" + id
				return
			}
			results <- id + "=" + ans["q"]
		}()
	}
	time.Sleep(50 * time.Millisecond)
	bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{AgentID: "x", RequestID: "req-x", Answers: map[string]string{"q": "1"}})
	bus.Publish(TopicAgentAskAnswer, AgentAskAnswer{AgentID: "y", RequestID: "req-y", Answers: map[string]string{"q": "2"}})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("waiters did not all resolve")
	}
	close(results)
	got := map[string]string{}
	for r := range results {
		if len(r) > 4 && r[:4] == "err:" {
			t.Fatalf("unexpected error: %s", r)
		}
		for i := 0; i < len(r); i++ {
			if r[i] == '=' {
				got[r[:i]] = r[i+1:]
				break
			}
		}
	}
	if got["x"] != "1" || got["y"] != "2" {
		t.Fatalf("got %v", got)
	}
}
