package starter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
)

// Errors returned by Pool methods.
var (
	ErrQueueFull = errors.New("queue is full")
	ErrClosed    = errors.New("pool is shut down")
	ErrNotFound  = errors.New("job not found")
	ErrFinished  = errors.New("job already finished")
)

var errTimeout = errors.New("job timed out")

// Pool runs jobs on a fixed number of workers fed by a bounded queue.
//
// TODO: add the state you need: a buffered queue channel of cfg.QueueSize, a
// mutex-protected map of jobs plus their creation order, a WaitGroup for the
// workers, and a context that Shutdown can cancel to stop running jobs.
type Pool struct {
	cfg Config
	log *slog.Logger
}

// NewPool starts cfg.Workers workers.
func NewPool(cfg Config) *Pool {
	// TODO: create the queue and start cfg.Workers goroutines.
	return &Pool{cfg: cfg, log: cfg.Logger}
}

// Submit enqueues a job without blocking. It returns ErrQueueFull when the
// queue has no free slot and ErrClosed after Shutdown. Ids start at 1.
func (p *Pool) Submit(_ string, _ json.RawMessage) (int, error) {
	// TODO
	return 0, errors.New("TODO: Submit is not implemented")
}

// Get returns a snapshot of the job.
func (p *Pool) Get(_ int) (JobView, bool) {
	// TODO
	return JobView{}, false
}

// List returns one page of jobs, newest first, and the total job count.
func (p *Pool) List(_, _ int) ([]JobView, int) {
	// TODO
	return []JobView{}, 0
}

// Cancel stops a queued or running job: ErrNotFound for unknown ids,
// ErrFinished when the job already finished.
func (p *Pool) Cancel(_ int) error {
	// TODO
	return errors.New("TODO: Cancel is not implemented")
}

// Shutdown stops accepting jobs and waits for queued and running jobs. If ctx
// ends first, the jobs still running are cancelled and ctx.Err() is returned.
func (p *Pool) Shutdown(_ context.Context) error {
	// TODO
	return nil
}
