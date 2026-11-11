package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const wait = 2 * time.Second

// manualTickers hands out channels the test fires by hand, keyed by interval.
type manualTickers struct {
	mu      sync.Mutex
	chans   map[time.Duration]chan time.Time
	created atomic.Int32
	stopped atomic.Int32
}

func (m *manualTickers) ch(d time.Duration) chan time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chans == nil {
		m.chans = map[time.Duration]chan time.Time{}
	}
	if m.chans[d] == nil {
		m.chans[d] = make(chan time.Time)
	}
	return m.chans[d]
}

func (m *manualTickers) New(d time.Duration) (<-chan time.Time, func()) {
	m.created.Add(1)
	return m.ch(d), func() { m.stopped.Add(1) }
}

// tick delivers one tick and fails if the scheduler is not listening for it.
func (m *manualTickers) tick(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case m.ch(d) <- time.Time{}:
	case <-time.After(wait):
		t.Fatalf("scheduler did not accept a tick for %v", d)
	}
}

type result struct {
	err error
}

// start runs the scheduler in the background; stop cancels it and returns Run's error.
func start(t *testing.T, s *Scheduler, jobs ...Job) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	go func() { done <- result{s.Run(ctx, jobs...)} }()
	var once sync.Once
	var err error
	stop = func() error {
		once.Do(func() {
			cancel()
			select {
			case r := <-done:
				err = r.err
			case <-time.After(wait):
				t.Error("Run did not return after cancellation")
			}
		})
		return err
	}
	t.Cleanup(func() { stop() })
	return stop
}

func recv(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(wait):
		t.Fatal("timed out waiting for the job")
	}
}

func TestRunsJobOnEveryTick(t *testing.T) {
	m := &manualTickers{}
	ran := make(chan struct{}, 10)
	s := &Scheduler{NewTicker: m.New}
	stop := start(t, s, Job{Name: "a", Every: time.Minute, Run: func(context.Context) error { ran <- struct{}{}; return nil }})

	for range 3 {
		m.tick(t, time.Minute)
		recv(t, ran)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if got := m.stopped.Load(); got != 1 {
		t.Fatalf("stopped %d tickers, want 1", got)
	}
}

func TestEveryJobUsesItsOwnInterval(t *testing.T) {
	m := &manualTickers{}
	fast, slow := make(chan struct{}, 10), make(chan struct{}, 10)
	s := &Scheduler{NewTicker: m.New}
	start(t, s,
		Job{Name: "fast", Every: time.Second, Run: func(context.Context) error { fast <- struct{}{}; return nil }},
		Job{Name: "slow", Every: time.Hour, Run: func(context.Context) error { slow <- struct{}{}; return nil }},
	)
	m.tick(t, time.Second)
	m.tick(t, time.Second)
	recv(t, fast)
	recv(t, fast)
	select {
	case <-slow:
		t.Fatal("the hourly job ran on the per-second ticks")
	default:
	}
	m.tick(t, time.Hour)
	recv(t, slow)
}

func TestSlowJobDoesNotBlockOthers(t *testing.T) {
	m := &manualTickers{}
	started, release := make(chan struct{}), make(chan struct{})
	quick := make(chan struct{}, 1)
	s := &Scheduler{NewTicker: m.New}
	start(t, s,
		Job{Name: "stuck", Every: time.Second, Run: func(ctx context.Context) error {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil
		}},
		Job{Name: "quick", Every: time.Minute, Run: func(context.Context) error { quick <- struct{}{}; return nil }},
	)
	m.tick(t, time.Second)
	recv(t, started)
	m.tick(t, time.Minute)
	recv(t, quick)
	close(release)
}

func TestJobNeverOverlapsItself(t *testing.T) {
	m := &manualTickers{}
	var running, maxRunning, calls atomic.Int32
	gate := make(chan struct{})
	entered := make(chan struct{}, 10)
	s := &Scheduler{NewTicker: m.New}
	stop := start(t, s, Job{Name: "serial", Every: time.Second, Run: func(ctx context.Context) error {
		n := running.Add(1)
		for {
			old := maxRunning.Load()
			if n <= old || maxRunning.CompareAndSwap(old, n) {
				break
			}
		}
		calls.Add(1)
		entered <- struct{}{}
		select {
		case <-gate:
		case <-ctx.Done():
		}
		running.Add(-1)
		return nil
	}})

	m.tick(t, time.Second)
	recv(t, entered)
	// Keep offering ticks while the first run is blocked.
	feederDone := make(chan struct{})
	go func() {
		defer close(feederDone)
		for range 3 {
			select {
			case m.ch(time.Second) <- time.Time{}:
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()
	<-feederDone
	if got := maxRunning.Load(); got != 1 {
		t.Fatalf("%d concurrent runs of the same job", got)
	}
	close(gate)
	stop()
	if got := maxRunning.Load(); got != 1 {
		t.Fatalf("%d concurrent runs of the same job", got)
	}
}

func TestErrorsAreReportedAndJobKeepsRunning(t *testing.T) {
	m := &manualTickers{}
	boom := errors.New("boom")
	type report struct {
		job string
		err error
	}
	reports := make(chan report, 10)
	var n atomic.Int32
	s := &Scheduler{NewTicker: m.New, OnError: func(job string, err error) { reports <- report{job, err} }}
	start(t, s, Job{Name: "flaky", Every: time.Second, Run: func(context.Context) error {
		if n.Add(1) == 1 {
			return boom
		}
		return nil
	}})
	m.tick(t, time.Second)
	select {
	case r := <-reports:
		if r.job != "flaky" || !errors.Is(r.err, boom) {
			t.Fatalf("report = %+v", r)
		}
	case <-time.After(wait):
		t.Fatal("error was not reported")
	}
	m.tick(t, time.Second)
	m.tick(t, time.Second) // accepted only if the second run already finished
	if got := n.Load(); got < 2 {
		t.Fatalf("job ran %d times, want it to keep its schedule", got)
	}
}

func TestPanicIsRecovered(t *testing.T) {
	m := &manualTickers{}
	reports := make(chan error, 10)
	ran := make(chan struct{}, 10)
	var n atomic.Int32
	s := &Scheduler{NewTicker: m.New, OnError: func(_ string, err error) { reports <- err }}
	start(t, s, Job{Name: "crashy", Every: time.Second, Run: func(context.Context) error {
		if n.Add(1) == 1 {
			panic("kaboom")
		}
		ran <- struct{}{}
		return nil
	}})
	m.tick(t, time.Second)
	select {
	case err := <-reports:
		if !errors.Is(err, ErrPanic) {
			t.Fatalf("error = %v, want ErrPanic", err)
		}
	case <-time.After(wait):
		t.Fatal("panic was not reported")
	}
	m.tick(t, time.Second)
	recv(t, ran)
}

func TestCancelWaitsForRunningJobAndStopsTickers(t *testing.T) {
	m := &manualTickers{}
	started := make(chan struct{})
	var finished atomic.Bool
	s := &Scheduler{NewTicker: m.New}
	stop := start(t, s,
		Job{Name: "a", Every: time.Second, Run: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			time.Sleep(10 * time.Millisecond)
			finished.Store(true)
			return ctx.Err()
		}},
		Job{Name: "b", Every: time.Minute, Run: func(context.Context) error { return nil }},
	)
	m.tick(t, time.Second)
	recv(t, started)
	if err := stop(); err != nil {
		t.Fatalf("Run returned %v after cancellation", err)
	}
	if !finished.Load() {
		t.Fatal("Run returned while a job was still running")
	}
	if got := m.stopped.Load(); got != 2 {
		t.Fatalf("stopped %d tickers, want 2", got)
	}
}

func TestInvalidJobsStartNothing(t *testing.T) {
	ok := func(context.Context) error { return nil }
	tests := map[string][]Job{
		"empty name":   {{Name: "", Every: time.Second, Run: ok}},
		"zero every":   {{Name: "a", Every: 0, Run: ok}},
		"negative":     {{Name: "a", Every: -time.Second, Run: ok}},
		"nil run":      {{Name: "a", Every: time.Second}},
		"duplicate":    {{Name: "a", Every: time.Second, Run: ok}, {Name: "a", Every: time.Minute, Run: ok}},
		"valid+broken": {{Name: "a", Every: time.Second, Run: ok}, {Name: "b", Every: 0, Run: ok}},
	}
	for name, jobs := range tests {
		t.Run(name, func(t *testing.T) {
			m := &manualTickers{}
			s := &Scheduler{NewTicker: m.New}
			err := s.Run(context.Background(), jobs...)
			if !errors.Is(err, ErrInvalidJob) {
				t.Fatalf("error = %v, want ErrInvalidJob", err)
			}
			if got := m.created.Load(); got != 0 {
				t.Fatalf("%d tickers created before validation finished", got)
			}
		})
	}
}

func TestZeroValueUsesRealTickers(t *testing.T) {
	var s Scheduler
	var n atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, Job{Name: "real", Every: time.Millisecond, Run: func(context.Context) error {
			if n.Add(1) == 3 {
				cancel()
			}
			return nil
		}})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(wait):
		t.Fatal("Run did not return")
	}
	if n.Load() < 3 {
		t.Fatalf("job ran %d times", n.Load())
	}
}

func TestNoJobsWaitsForCancellation(t *testing.T) {
	var s Scheduler
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("Run returned before the context ended")
	}
}
