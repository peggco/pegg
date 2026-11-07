package agent

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/core/event"
)

func NewAskRequestID() string { return uuid.NewString() }

type askWaiter struct {
	agentID   string
	requestID string
	ch        chan map[string]string
}

var askHub = struct {
	sync.Mutex
	buses   map[event.Bus]struct{}
	waiters map[chan map[string]string]*askWaiter
}{
	buses:   make(map[event.Bus]struct{}),
	waiters: make(map[chan map[string]string]*askWaiter),
}

func ensureAskHub(bus event.Bus) {
	if bus == nil {
		return
	}
	askHub.Lock()
	defer askHub.Unlock()
	if _, ok := askHub.buses[bus]; ok {
		return
	}
	askHub.buses[bus] = struct{}{}
	_ = bus.Subscribe(TopicAgentAskAnswer, dispatchAskAnswer)
}

func dispatchAskAnswer(e AgentAskAnswer) {
	askHub.Lock()
	chans := make([]chan map[string]string, 0, len(askHub.waiters))
	for ch, w := range askHub.waiters {
		if w.agentID != e.AgentID {
			continue
		}
		if e.RequestID != "" && w.requestID != "" && e.RequestID != w.requestID {
			continue
		}
		chans = append(chans, ch)
	}
	askHub.Unlock()
	for _, ch := range chans {
		select {
		case ch <- e.Answers:
		default:
		}
	}
}

func Ask(ctx context.Context, bus event.Bus, agentID, requestID string, publish func()) (map[string]string, error) {
	ensureAskHub(bus)
	ch := make(chan map[string]string, 1)
	askHub.Lock()
	askHub.waiters[ch] = &askWaiter{agentID: agentID, requestID: requestID, ch: ch}
	askHub.Unlock()
	defer func() {
		askHub.Lock()
		delete(askHub.waiters, ch)
		askHub.Unlock()
	}()
	if publish != nil {
		publish()
	}
	select {
	case answers := <-ch:
		if answers == nil {
			answers = map[string]string{}
		}
		return answers, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
