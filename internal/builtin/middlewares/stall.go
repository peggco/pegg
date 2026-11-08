package middlewares

import (
	"context"
	"errors"
	"sync"
	"time"

	agentmw "github.com/peggco/pegg/internal/agent/middleware"
	"github.com/peggco/pegg/internal/llm"
)

const (
	DefaultStallTimeout = 10 * time.Second
	stallCancelGrace    = time.Second
)

var (
	errStallTimeout     = errors.New("provider request stalled: no response within the idle window")
	errStallAfterOutput = errors.New("provider stream stalled after partial output; not retrying to avoid duplicate content")
)

type StallOption func(*Stall)

func WithStallTimeout(d time.Duration) StallOption {
	return func(s *Stall) {
		if d > 0 {
			s.timeout = d
		}
	}
}

func WithStallMaxRetries(n int) StallOption {
	return func(s *Stall) {
		if n >= 0 {
			s.maxRetries = n
		}
	}
}

type Stall struct {
	agentmw.BaseMiddleware
	timeout    time.Duration
	maxRetries int
}

func NewStall(opts ...StallOption) *Stall {
	s := &Stall{timeout: DefaultStallTimeout}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Stall) InvokeLLMStream(ctx context.Context, req *llm.Request, handler llm.StreamHandler, next agentmw.LLMStreamInvoker) error {
	for attempt := 0; ; attempt++ {
		err, stalled := s.runStream(ctx, req, handler, next)
		if !stalled {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.maxRetries > 0 && attempt >= s.maxRetries {
			return err
		}
	}
}

func (s *Stall) runStream(ctx context.Context, req *llm.Request, handler llm.StreamHandler, next agentmw.LLMStreamInvoker) (error, bool) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	lastActivity := time.Now()
	anyDelivered := false
	finished := false
	abandoned := false

	wrapped := func(chunk llm.StreamChunk) error {
		mu.Lock()
		if abandoned {
			mu.Unlock()
			return nil
		}
		lastActivity = time.Now()
		if chunk.Content != "" || chunk.Reasoning != "" || len(chunk.ToolCalls) > 0 {
			anyDelivered = true
		}
		if chunk.IsDone || (chunk.FinishReason != "" && chunk.FinishReason != llm.FinishReasonNull) {
			finished = true
		}
		mu.Unlock()
		return handler(chunk)
	}

	done := make(chan error, 1)
	go func() {
		done <- next(attemptCtx, req, wrapped)
	}()

	ticker := time.NewTicker(stallTick(s.timeout))
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			return err, false
		case <-ticker.C:
			mu.Lock()
			if finished {
				abandoned = true
				mu.Unlock()
				cancel()
				waitFor(done, stallCancelGrace)
				return nil, false
			}
			if time.Since(lastActivity) < s.timeout {
				mu.Unlock()
				continue
			}
			if anyDelivered {
				abandoned = true
				mu.Unlock()
				cancel()
				waitFor(done, stallCancelGrace)
				return errStallAfterOutput, false
			}
			abandoned = true
			mu.Unlock()
			cancel()
			waitFor(done, stallCancelGrace)
			return errStallTimeout, true
		case <-ctx.Done():
			cancel()
			waitFor(done, stallCancelGrace)
			return ctx.Err(), false
		}
	}
}

func stallTick(timeout time.Duration) time.Duration {
	tick := timeout / 10
	if tick < 100*time.Millisecond {
		tick = 100 * time.Millisecond
	}
	if tick > time.Second {
		tick = time.Second
	}
	return tick
}

func waitFor[T any](done <-chan T, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}
