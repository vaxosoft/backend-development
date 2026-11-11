// Timers, tickers and AfterFunc: the correct way to wait, repeat and give up.
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

func main() {
	// Ticker: always Stop it, or it keeps firing for the life of the program.
	ticker := time.NewTicker(5 * time.Millisecond)
	ticks := 0
	for range ticker.C {
		ticks++
		if ticks == 3 {
			ticker.Stop()
			break
		}
	}
	fmt.Println("ticks:", ticks)

	// Timer: Stop reports whether it prevented the firing. Since Go 1.23 Stop and
	// Reset also guarantee no stale value is left in t.C.
	t := time.NewTimer(time.Hour)
	fmt.Println("stopped before firing:", t.Stop())
	t.Reset(5 * time.Millisecond)
	<-t.C
	fmt.Println("timer fired after Reset")

	// AfterFunc runs a function in its own goroutine; Stop cancels it.
	var fired atomic.Int32
	done := make(chan struct{})
	time.AfterFunc(5*time.Millisecond, func() { fired.Add(1); close(done) })
	cancelled := time.AfterFunc(time.Hour, func() { fired.Add(100) })
	fmt.Println("cancelled AfterFunc:", cancelled.Stop())
	<-done
	fmt.Println("AfterFunc calls:", fired.Load())

	// A polling loop with an overall deadline and one reusable timer per wait.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	attempts := 0
	err := poll(ctx, 5*time.Millisecond, func() bool { attempts++; return attempts == 3 })
	fmt.Println("poll succeeded after", attempts, "attempts, err:", err)
	err = poll(ctx, 5*time.Millisecond, func() bool { return false })
	fmt.Println("poll never ready:", err)

	// Debounce-style idle timeout: each event resets the timer.
	events := make(chan string)
	go func() {
		for _, e := range []string{"a", "b", "c"} {
			events <- e
			time.Sleep(time.Millisecond)
		}
	}()
	idle := time.NewTimer(30 * time.Millisecond)
	defer idle.Stop()
	for got := 0; got < 3; {
		select {
		case e := <-events:
			got++
			fmt.Println("event", e)
			idle.Reset(30 * time.Millisecond)
		case <-idle.C:
			fmt.Println("idle timeout")
			return
		}
	}
	fmt.Println("all events seen before the idle timeout")
}

// poll calls ready every interval until it returns true or ctx ends.
// One Timer is reused instead of calling time.After on every iteration.
func poll(ctx context.Context, interval time.Duration, ready func() bool) error {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if ready() {
				return nil
			}
			timer.Reset(interval)
		}
	}
}
