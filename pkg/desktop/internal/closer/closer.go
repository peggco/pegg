package closer

import (
	"context"
	"sync"
	"time"
)

const (
	defaultDeadline  = 2 * time.Second
	defaultThreshold = 300 * time.Millisecond
)

type Task struct {
	Name string
	Run  func(context.Context) error
}

type Options struct {
	Deadline  time.Duration
	Threshold time.Duration
	Logf      func(format string, args ...any)
	OnSlow    func()
}

type Outcome struct {
	Duration time.Duration
	TimedOut bool
	Slow     bool
}

func Run(tasks []Task, opts Options) Outcome {
	deadline := opts.Deadline
	if deadline <= 0 {
		deadline = defaultDeadline
	}
	threshold := opts.Threshold
	if threshold <= 0 {
		threshold = defaultThreshold
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	var wg sync.WaitGroup
	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task.Run(ctx); err != nil {
				logf("shutdown: %s: %v", task.Name, err)
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(threshold)
	defer timer.Stop()
	slow := timer.C
	outcome := Outcome{}
	for {
		select {
		case <-slow:
			slow = nil
			select {
			case <-done:
			default:
				outcome.Slow = true
				if opts.OnSlow != nil {
					opts.OnSlow()
				}
			}
		case <-done:
			outcome.Duration = time.Since(started)
			logf("shutdown: completed in %s", outcome.Duration)
			return outcome
		case <-ctx.Done():
			outcome.Duration = time.Since(started)
			outcome.TimedOut = true
			logf("shutdown: deadline %s exceeded", deadline)
			return outcome
		}
	}
}
