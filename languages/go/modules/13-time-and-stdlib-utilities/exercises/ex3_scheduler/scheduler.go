// Package scheduler is exercise 3: an in-process periodic job runner.
//
// Run the tests: go test ./modules/13-time-and-stdlib-utilities/exercises/ex3_scheduler/
package scheduler

import (
	"context"
	"errors"
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
	// NewTicker is replaced in tests by a manual ticker. When nil, use time.NewTicker.
	NewTicker func(d time.Duration) (ticks <-chan time.Time, stop func())
	// OnError, if set, receives job errors and recovered panics.
	OnError func(job string, err error)
}

// Run executes every job on its own interval until ctx is done.
//
// Requirements:
//   - Validate first: an empty or duplicate Name, Every <= 0 or a nil Run
//     makes Run return an error wrapping ErrInvalidJob before any ticker is
//     created or any job started.
//   - Each job has its own ticker and goroutine; one slow job must not delay another.
//   - A job never overlaps with itself: while it runs, later ticks are not run concurrently.
//   - A returned error is passed to OnError (job name, error) and the job keeps its schedule.
//   - A panic is recovered, reported to OnError as an error wrapping ErrPanic, and the job keeps its schedule.
//   - Run blocks until ctx is done, even when there are no jobs.
//   - When ctx is done: stop every ticker, wait for jobs that are running
//     (they receive ctx too) and return nil. No goroutine may outlive Run.
func (s *Scheduler) Run(ctx context.Context, jobs ...Job) error {
	panic("TODO")
}
