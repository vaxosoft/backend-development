// Package scheduler is exercise 3: an in-process periodic job runner.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrInvalidJob is returned by Run when a job cannot be scheduled.
	ErrInvalidJob = errors.New("scheduler: invalid job")
	// ErrPanic wraps the value recovered from a panicking job.
	ErrPanic = errors.New("scheduler: job panicked")
)

// Job is a named function run every Every.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(ctx context.Context) error
}

// Scheduler runs jobs on tickers. The zero value uses real time.Ticker values.
type Scheduler struct {
	// NewTicker is replaced in tests by a manual ticker.
	NewTicker func(d time.Duration) (ticks <-chan time.Time, stop func())
	// OnError, if set, receives job errors and recovered panics.
	OnError func(job string, err error)
}

func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Run blocks until ctx is done and every running job has returned.
func (s *Scheduler) Run(ctx context.Context, jobs ...Job) error {
	seen := map[string]bool{}
	for _, j := range jobs {
		if j.Name == "" || j.Every <= 0 || j.Run == nil || seen[j.Name] {
			return fmt.Errorf("%w: %q", ErrInvalidJob, j.Name)
		}
		seen[j.Name] = true
	}
	newTicker := s.NewTicker
	if newTicker == nil {
		newTicker = realTicker
	}

	var wg sync.WaitGroup
	for _, j := range jobs {
		ticks, stop := newTicker(j.Every)
		wg.Go(func() {
			defer stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticks:
					if err := s.runOnce(ctx, j); err != nil && s.OnError != nil {
						s.OnError(j.Name, err)
					}
				}
			}
		})
	}
	<-ctx.Done() // also covers the no-jobs case
	wg.Wait()
	return nil
}

func (s *Scheduler) runOnce(ctx context.Context, j Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrPanic, r)
		}
	}()
	return j.Run(ctx)
}
