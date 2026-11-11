package firstsuccess

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var (
	errA = errors.New("a failed")
	errB = errors.New("b failed")
)

// blocker waits for cancellation, like a slow network call that honours its context.
func blocker(running, sawCancel *atomic.Int32) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		running.Add(1)
		defer running.Add(-1)
		select {
		case <-ctx.Done():
			sawCancel.Add(1)
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "too slow", nil
		}
	}
}

func ok(v string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return v, nil }
}

func fail(err error) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return "", err }
}

func TestFastSuccessWinsAndCancelsLosers(t *testing.T) {
	var running, sawCancel atomic.Int32
	slow := blocker(&running, &sawCancel)
	winner := func(context.Context) (string, error) {
		for range 2000 { // succeed once both slow attempts are really running
			if running.Load() == 2 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		return "fast", nil
	}
	got, err := FirstSuccess(context.Background(), slow, slow, winner)
	if err != nil || got != "fast" {
		t.Fatalf("got %q, %v", got, err)
	}
	if running.Load() != 0 {
		t.Fatal("FirstSuccess returned while attempts were still running")
	}
	if sawCancel.Load() != 2 {
		t.Fatalf("%d of 2 losers saw cancellation", sawCancel.Load())
	}
}

func TestFailuresAreIgnoredWhileOthersMaySucceed(t *testing.T) {
	late := func(context.Context) (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "late ok", nil
	}
	got, err := FirstSuccess(context.Background(), fail(errA), fail(errB), late)
	if err != nil || got != "late ok" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAllFailJoinsErrors(t *testing.T) {
	_, err := FirstSuccess(context.Background(), fail(errA), fail(errB))
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("error should wrap both failures, got %v", err)
	}
}

func TestParentCancellation(t *testing.T) {
	var running, sawCancel atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(5*time.Millisecond, cancel)
	_, err := FirstSuccess(ctx, blocker(&running, &sawCancel), blocker(&running, &sawCancel))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if running.Load() != 0 {
		t.Fatal("attempts still running after return")
	}
}

func TestNoFunctions(t *testing.T) {
	if _, err := FirstSuccess[string](context.Background()); !errors.Is(err, ErrNoFunctions) {
		t.Fatalf("got %v", err)
	}
}

func TestManySuccessesReturnOne(t *testing.T) {
	for range 50 {
		got, err := FirstSuccess(context.Background(), ok("x"), ok("x"), ok("x"), ok("x"))
		if err != nil || got != "x" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
}
