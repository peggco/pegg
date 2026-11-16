package closer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunTimesOutAtDeadline(t *testing.T) {
	start := time.Now()
	out := Run([]Task{{
		Name: "slow",
		Run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}}, Options{Deadline: 50 * time.Millisecond, Threshold: 10 * time.Millisecond})

	if !out.TimedOut {
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("run took %s, expected it to stop at the deadline", elapsed)
	}
}

func TestRunWaitsForAllFastTasks(t *testing.T) {
	var ran atomic.Int32
	tasks := []Task{
		{Name: "one", Run: func(context.Context) error { ran.Add(1); return nil }},
		{Name: "two", Run: func(context.Context) error { ran.Add(1); return nil }},
	}
	out := Run(tasks, Options{Deadline: time.Second, Threshold: 100 * time.Millisecond})
	if out.TimedOut {
		t.Fatal("unexpected timeout")
	}
	if ran.Load() != 2 {
		t.Fatalf("ran = %d, want 2", ran.Load())
	}
	if out.Slow {
		t.Fatal("fast tasks should not be marked slow")
	}
}

func TestOnSlowFiresOnceAfterThreshold(t *testing.T) {
	var calls atomic.Int32
	var at atomic.Int64
	start := time.Now()
	out := Run([]Task{{
		Name: "slow",
		Run: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
			case <-time.After(150 * time.Millisecond):
			}
			return nil
		},
	}}, Options{
		Deadline:  time.Second,
		Threshold: 20 * time.Millisecond,
		OnSlow: func() {
			calls.Add(1)
			at.Store(int64(time.Since(start)))
		},
	})

	if !out.Slow {
		t.Fatal("expected slow outcome")
	}
	if calls.Load() != 1 {
		t.Fatalf("onSlow calls = %d, want 1", calls.Load())
	}
	if delay := time.Duration(at.Load()); delay < 20*time.Millisecond {
		t.Fatalf("onSlow fired after %s, before the threshold", delay)
	}
}

func TestOnSlowNotCalledForFastTasks(t *testing.T) {
	var calls atomic.Int32
	out := Run([]Task{{
		Name: "fast",
		Run:  func(context.Context) error { return nil },
	}}, Options{
		Deadline:  time.Second,
		Threshold: 100 * time.Millisecond,
		OnSlow:    func() { calls.Add(1) },
	})

	if out.Slow {
		t.Fatal("fast task marked slow")
	}
	if calls.Load() != 0 {
		t.Fatalf("onSlow calls = %d, want 0", calls.Load())
	}
}
