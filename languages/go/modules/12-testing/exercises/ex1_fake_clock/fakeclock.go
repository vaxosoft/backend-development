// Package fakeclock is exercise 1: a clock that only moves when the test says so.
//
// Run the tests: go test -race ./modules/12-testing/exercises/ex1_fake_clock/
package fakeclock

import "time"

// Clock is what production code depends on instead of calling time.Now and time.After directly.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Fake is a Clock under the test's control. It must be safe for concurrent use.
type Fake struct {
	now time.Time
}

// New returns a Fake that starts at start.
func New(start time.Time) *Fake {
	return &Fake{now: start}
}

// Now returns the current fake time.
func (f *Fake) Now() time.Time {
	return time.Time{}
}

// Since returns the fake time elapsed since t.
func (f *Fake) Since(t time.Time) time.Duration {
	return 0
}

// Set jumps to t. Moving forward fires the timers that become due, like Advance; moving backward
// fires nothing.
func (f *Fake) Set(t time.Time) {
}

// Advance moves the clock forward by d and fires every timer whose deadline is now reached.
// A negative d is a programming error: panic.
func (f *Fake) Advance(d time.Duration) {
}

// After returns a channel that receives the deadline (Now() + d at the time of the call) once the
// clock reaches it. The channel is buffered so Advance never blocks. If d <= 0 the channel already
// holds the current time.
func (f *Fake) After(d time.Duration) <-chan time.Time {
	return nil
}

// Pending reports how many timers have not fired yet; tests use it to wait until a goroutine has
// started waiting before they advance time.
func (f *Fake) Pending() int {
	return -1
}
