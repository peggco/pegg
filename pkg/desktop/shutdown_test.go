//go:build gui

package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunShutdownStepsIsParallel(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	step := func(context.Context) { time.Sleep(100 * time.Millisecond) }
	start := time.Now()
	a.runShutdownSteps(ctx, []shutdownStep{
		{name: "one", run: step},
		{name: "two", run: step},
		{name: "three", run: step},
	})
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("three 100ms steps took %s, want them to run in parallel", elapsed)
	}
}

func TestRunShutdownStepsHonorsDeadline(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var started atomic.Int32
	step := func(ctx context.Context) {
		started.Add(1)
		<-ctx.Done()
	}
	start := time.Now()
	a.runShutdownSteps(ctx, []shutdownStep{
		{name: "one", run: step},
		{name: "two", run: step},
		{name: "three", run: step},
	})
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("shutdown steps took %s, want the deadline to bound them", elapsed)
	}
	if got := started.Load(); got != 3 {
		t.Fatalf("started steps = %d, want 3", got)
	}
}
