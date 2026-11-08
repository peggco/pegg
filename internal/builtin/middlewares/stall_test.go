package middlewares

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/llm"
)

const stallTestTimeout = 50 * time.Millisecond

func TestStallStreamRetriesOnSilence(t *testing.T) {
	s := NewStall(WithStallTimeout(stallTestTimeout))
	req := llm.NewRequest("m", nil)

	var calls atomic.Int32
	var chunks []string
	err := s.InvokeLLMStream(context.Background(), req, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk.Content)
		return nil
	}, func(ctx context.Context, _ *llm.Request, handler llm.StreamHandler) error {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			return ctx.Err()
		}
		return handler(llm.StreamChunk{Content: "ok"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Fatalf("chunks = %v, want [ok]", chunks)
	}
}

func TestStallStreamMaxRetries(t *testing.T) {
	s := NewStall(WithStallTimeout(stallTestTimeout), WithStallMaxRetries(1))
	req := llm.NewRequest("m", nil)

	var calls atomic.Int32
	err := s.InvokeLLMStream(context.Background(), req, func(llm.StreamChunk) error { return nil }, func(ctx context.Context, _ *llm.Request, _ llm.StreamHandler) error {
		calls.Add(1)
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, errStallTimeout) {
		t.Fatalf("expected errStallTimeout, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestStallStreamNoRetryAfterOutput(t *testing.T) {
	s := NewStall(WithStallTimeout(stallTestTimeout))
	req := llm.NewRequest("m", nil)

	var calls atomic.Int32
	var chunks []string
	err := s.InvokeLLMStream(context.Background(), req, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk.Content)
		return nil
	}, func(ctx context.Context, _ *llm.Request, handler llm.StreamHandler) error {
		calls.Add(1)
		if err := handler(llm.StreamChunk{Content: "partial"}); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, errStallAfterOutput) {
		t.Fatalf("expected errStallAfterOutput, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
	if len(chunks) != 1 || chunks[0] != "partial" {
		t.Fatalf("chunks = %v, want [partial]", chunks)
	}
}

func TestStallStreamFinishedChunk(t *testing.T) {
	s := NewStall(WithStallTimeout(stallTestTimeout))
	req := llm.NewRequest("m", nil)

	var calls atomic.Int32
	err := s.InvokeLLMStream(context.Background(), req, func(llm.StreamChunk) error { return nil }, func(ctx context.Context, _ *llm.Request, handler llm.StreamHandler) error {
		calls.Add(1)
		if err := handler(llm.StreamChunk{Content: "done", IsDone: true}); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}

func TestStallStreamNullFinishReasonIsNotDone(t *testing.T) {
	s := NewStall(WithStallTimeout(stallTestTimeout))
	req := llm.NewRequest("m", nil)

	var calls atomic.Int32
	err := s.InvokeLLMStream(context.Background(), req, func(llm.StreamChunk) error { return nil }, func(ctx context.Context, _ *llm.Request, handler llm.StreamHandler) error {
		if calls.Add(1) == 1 {
			if err := handler(llm.StreamChunk{FinishReason: llm.FinishReasonNull}); err != nil {
				return err
			}
			<-ctx.Done()
			return ctx.Err()
		}
		return handler(llm.StreamChunk{Content: "ok", IsDone: true})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestStallStreamContextCancel(t *testing.T) {
	s := NewStall(WithStallTimeout(time.Hour))
	req := llm.NewRequest("m", nil)

	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := s.InvokeLLMStream(ctx, req, func(llm.StreamChunk) error { return nil }, func(ctx context.Context, _ *llm.Request, _ llm.StreamHandler) error {
		calls.Add(1)
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}

func TestStallStreamHandlerError(t *testing.T) {
	s := NewStall(WithStallTimeout(time.Hour))
	req := llm.NewRequest("m", nil)

	boom := errors.New("handler failed")
	var calls atomic.Int32
	err := s.InvokeLLMStream(context.Background(), req, func(llm.StreamChunk) error {
		return boom
	}, func(_ context.Context, _ *llm.Request, handler llm.StreamHandler) error {
		calls.Add(1)
		return handler(llm.StreamChunk{Content: "x"})
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}
