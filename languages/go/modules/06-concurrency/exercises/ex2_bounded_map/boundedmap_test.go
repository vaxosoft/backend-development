package boundedmap

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func TestResultsKeepInputOrder(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	got, err := Map(context.Background(), 4, in, func(ctx context.Context, n int) (string, error) {
		time.Sleep(time.Duration(7-n) * time.Millisecond) // later items finish first
		return string(rune('a' + n - 1)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "c", "d", "e", "f"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEmptyInput(t *testing.T) {
	got, err := Map(context.Background(), 3, []int{}, func(context.Context, int) (int, error) { return 0, nil })
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestNeverExceedsLimit(t *testing.T) {
	var running, peak atomic.Int32
	_, err := Map(context.Background(), 3, make([]int, 12), func(ctx context.Context, _ int) (int, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(3 * time.Millisecond)
		return 0, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if p := peak.Load(); p > 3 {
		t.Fatalf("peak concurrency %d exceeds limit 3", p)
	}
}

func TestRunsConcurrently(t *testing.T) {
	// Three calls rendezvous: this only completes if all three run at once.
	var arrived atomic.Int32
	all := make(chan struct{})
	_, err := Map(context.Background(), 3, []int{1, 2, 3}, func(ctx context.Context, n int) (int, error) {
		if arrived.Add(1) == 3 {
			close(all)
		}
		select {
		case <-all:
			return n, nil
		case <-time.After(2 * time.Second):
			return 0, errors.New("calls did not overlap")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirstErrorCancelsTheRest(t *testing.T) {
	var sawCancel atomic.Int32
	started := make(chan struct{}, 3)
	_, err := Map(context.Background(), 4, []int{1, 2, 3, 4}, func(ctx context.Context, n int) (int, error) {
		if n == 1 {
			for range 3 { // fail only once the others are really running
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					return 0, errors.New("other calls never started")
				}
			}
			return 0, errBoom
		}
		started <- struct{}{}
		select {
		case <-ctx.Done():
			sawCancel.Add(1)
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
			return n, nil
		}
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("want errBoom, got %v", err)
	}
	if got := sawCancel.Load(); got != 3 {
		t.Fatalf("%d of 3 other calls saw the cancellation before Map returned", got)
	}
}

func TestNoNewCallsAfterError(t *testing.T) {
	var calls atomic.Int32
	_, err := Map(context.Background(), 1, []int{1, 2, 3, 4, 5}, func(ctx context.Context, n int) (int, error) {
		calls.Add(1)
		return 0, errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("fn called %d times, want 1", got)
	}
}

func TestAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	_, err := Map(ctx, 2, []int{1, 2, 3}, func(context.Context, int) (int, error) {
		calls.Add(1)
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("fn must not be called")
	}
}

func TestLimitBelowOneMeansOne(t *testing.T) {
	var running, peak atomic.Int32
	got, err := Map(context.Background(), 0, []int{1, 2, 3}, func(ctx context.Context, n int) (int, error) {
		if r := running.Add(1); r > peak.Load() {
			peak.Store(r)
		}
		defer running.Add(-1)
		time.Sleep(time.Millisecond)
		return n * 2, nil
	})
	if err != nil || !slices.Equal(got, []int{2, 4, 6}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if peak.Load() != 1 {
		t.Fatalf("peak = %d, want 1", peak.Load())
	}
}
