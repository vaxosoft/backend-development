// context: cancellation, deadlines, and goroutines that stop when asked.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// ticker counts until ctx is cancelled and reports how many ticks it did.
func ticker(ctx context.Context, every time.Duration, ticks *atomic.Int32) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ticks.Add(1)
			}
		}
	}()
	return stopped
}

// slowQuery pretends to do work for d but honours cancellation.
func slowQuery(ctx context.Context, d time.Duration) (string, error) {
	select {
	case <-time.After(d):
		return "rows", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type ctxKey struct{}

func main() {
	// Cancel: the goroutine exits and we can wait for that to happen.
	ctx, cancel := context.WithCancel(context.Background())
	var ticks atomic.Int32
	stopped := ticker(ctx, time.Millisecond, &ticks)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-stopped
	fmt.Println("ticker stopped, ticked at least once:", ticks.Load() > 0)

	// Timeout: the caller gives up after 10ms; the callee notices.
	tctx, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2() // always release the timer
	_, err := slowQuery(tctx, time.Second)
	fmt.Println("timeout:", err, errors.Is(err, context.DeadlineExceeded))

	res, err := slowQuery(context.Background(), time.Millisecond)
	fmt.Println("fast query:", res, err)

	// Cancellation propagates to children, and carries a cause.
	parent, cancelParent := context.WithCancelCause(context.Background())
	child, cancelChild := context.WithTimeout(parent, time.Hour)
	defer cancelChild()
	cancelParent(errors.New("shutting down"))
	<-child.Done()
	fmt.Println("child:", child.Err(), "| cause:", context.Cause(child))

	// Values are for request-scoped data (request id), not for optional parameters.
	vctx := context.WithValue(context.Background(), ctxKey{}, "req-123")
	fmt.Println("request id:", vctx.Value(ctxKey{}))

	// AfterFunc runs cleanup when the context ends.
	actx, cancel3 := context.WithCancel(context.Background())
	cleaned := make(chan string, 1)
	context.AfterFunc(actx, func() { cleaned <- "cleaned up" })
	cancel3()
	fmt.Println(<-cleaned)
}
