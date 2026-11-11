package starter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Errors returned by Pool methods.
var (
	ErrQueueFull = errors.New("queue is full")
	ErrClosed    = errors.New("pool is shut down")
	ErrNotFound  = errors.New("job not found")
	ErrFinished  = errors.New("job already finished")
)

var errTimeout = errors.New("job timed out")

type job struct {
	view    JobView
	payload json.RawMessage
	cancel  context.CancelFunc // set while the job runs
}

// Pool runs jobs on a fixed number of workers fed by a bounded queue.
type Pool struct {
	cfg   Config
	log   *slog.Logger
	queue chan *job
	wg    sync.WaitGroup

	// baseCtx is the parent of every running job; stop cancels them all.
	baseCtx context.Context
	stop    context.CancelFunc

	mu     sync.Mutex // guards everything below and every job.view
	jobs   map[int]*job
	order  []int // ids, oldest first
	nextID int
	closed bool
}

// NewPool starts cfg.Workers workers.
func NewPool(cfg Config) *Pool {
	ctx, stop := context.WithCancel(context.Background())
	p := &Pool{
		cfg:     cfg,
		log:     cfg.Logger,
		queue:   make(chan *job, cfg.QueueSize),
		baseCtx: ctx,
		stop:    stop,
		jobs:    map[int]*job{},
		nextID:  1,
	}
	for range cfg.Workers {
		p.wg.Go(p.worker)
	}
	return p
}

// Submit enqueues a job without blocking. It returns ErrQueueFull when the
// queue has no free slot and ErrClosed after Shutdown.
func (p *Pool) Submit(jobType string, payload json.RawMessage) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, ErrClosed
	}
	j := &job{
		view:    JobView{ID: p.nextID, Type: jobType, Status: StatusQueued},
		payload: payload,
	}
	select {
	case p.queue <- j:
	default:
		return 0, ErrQueueFull
	}
	p.jobs[j.view.ID] = j
	p.order = append(p.order, j.view.ID)
	p.nextID++
	return j.view.ID, nil
}

// Get returns a snapshot of the job.
func (p *Pool) Get(id int) (JobView, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	j, ok := p.jobs[id]
	if !ok {
		return JobView{}, false
	}
	return j.view, true
}

// List returns one page of jobs, newest first, and the total job count.
func (p *Pool) List(limit, offset int) ([]JobView, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := len(p.order)
	items := []JobView{}
	for i := total - 1 - offset; i >= 0 && len(items) < limit; i-- {
		items = append(items, p.jobs[p.order[i]].view)
	}
	return items, total
}

// Cancel stops a queued or running job.
func (p *Pool) Cancel(id int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	j, ok := p.jobs[id]
	if !ok {
		return ErrNotFound
	}
	if j.view.Status.Finished() {
		return ErrFinished
	}
	j.view.Status = StatusCancelled
	if j.cancel != nil {
		j.cancel()
	}
	return nil
}

// Shutdown stops accepting jobs and waits for queued and running jobs. If ctx
// ends first, the jobs still running are cancelled and ctx.Err() is returned.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		p.stop()
		<-done
		return ctx.Err()
	}
}

func (p *Pool) worker() {
	for j := range p.queue {
		p.run(j)
	}
}

func (p *Pool) run(j *job) {
	ctx, cancel := context.WithCancel(p.baseCtx)
	defer cancel()

	p.mu.Lock()
	if j.view.Status == StatusCancelled {
		p.mu.Unlock()
		return
	}
	j.view.Status = StatusRunning
	j.cancel = cancel
	id, jobType := j.view.ID, j.view.Type
	p.mu.Unlock()

	log := p.log.With("job_id", id, "type", jobType)
	for attempt := 1; ; attempt++ {
		p.update(j, func(v *JobView) { v.Attempts = attempt })
		result, err := p.attempt(ctx, j)
		switch {
		case err == nil:
			log.Info("job succeeded", "attempts", attempt)
			p.update(j, func(v *JobView) { v.Status, v.Result = StatusSucceeded, &result })
			return
		case ctx.Err() != nil:
			log.Info("job cancelled", "attempts", attempt)
			p.update(j, func(v *JobView) { v.Status = StatusCancelled })
			return
		case attempt >= p.cfg.MaxAttempts:
			msg := err.Error()
			log.Warn("job failed", "attempts", attempt, "error", msg)
			p.update(j, func(v *JobView) { v.Status, v.Error = StatusFailed, &msg })
			return
		}
		log.Warn("job attempt failed, retrying", "attempt", attempt, "error", err)
		if !sleep(ctx, p.cfg.RetryDelay) {
			p.update(j, func(v *JobView) { v.Status = StatusCancelled })
			return
		}
	}
}

// attempt runs the job once under its own timeout.
func (p *Pool) attempt(ctx context.Context, j *job) (string, error) {
	actx, cancel := context.WithTimeout(ctx, p.cfg.JobTimeout)
	defer cancel()
	result, err := execute(actx, j.view.Type, j.payload)
	if errors.Is(err, context.DeadlineExceeded) {
		err = errTimeout
	}
	return result, err
}

// update applies fn under the lock unless the job was already cancelled.
func (p *Pool) update(j *job, fn func(*JobView)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if j.view.Status == StatusCancelled {
		return
	}
	fn(&j.view)
}

// sleep waits for d and reports false if ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
