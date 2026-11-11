package fakeclock

import (
	"testing"
	"time"
)

var start = time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

var _ Clock = (*Fake)(nil)

func receive(t *testing.T, ch <-chan time.Time) (time.Time, bool) {
	t.Helper()
	select {
	case v := <-ch:
		return v, true
	default:
		return time.Time{}, false
	}
}

func TestNowSetAndSince(t *testing.T) {
	f := New(start)
	if !f.Now().Equal(start) {
		t.Fatalf("Now = %v", f.Now())
	}
	f.Advance(90 * time.Minute)
	if got := f.Since(start); got != 90*time.Minute {
		t.Fatalf("Since = %v", got)
	}
	f.Set(start.Add(24 * time.Hour))
	if !f.Now().Equal(start.Add(24 * time.Hour)) {
		t.Fatalf("Now after Set = %v", f.Now())
	}
	f.Set(start) // backwards is allowed for tests that simulate clock changes
	if !f.Now().Equal(start) {
		t.Fatalf("Now after Set back = %v", f.Now())
	}
}

func TestAfterFiresOnlyWhenTheDeadlineIsReached(t *testing.T) {
	f := New(start)
	ch := f.After(time.Minute)
	if f.Pending() != 1 {
		t.Fatalf("Pending = %d, want 1", f.Pending())
	}

	f.Advance(59 * time.Second)
	if _, ok := receive(t, ch); ok {
		t.Fatal("fired 1s early")
	}
	f.Advance(time.Second)
	got, ok := receive(t, ch)
	if !ok {
		t.Fatal("did not fire at the deadline")
	}
	if !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("received %v, want the deadline %v", got, start.Add(time.Minute))
	}
	if f.Pending() != 0 {
		t.Fatalf("Pending = %d after firing", f.Pending())
	}
}

func TestOneBigAdvanceFiresEveryDueTimer(t *testing.T) {
	f := New(start)
	short, long, never := f.After(time.Minute), f.After(time.Hour), f.After(48*time.Hour)
	f.Advance(2 * time.Hour)
	if got, ok := receive(t, short); !ok || !got.Equal(start.Add(time.Minute)) {
		t.Errorf("short timer: %v %v", got, ok)
	}
	if got, ok := receive(t, long); !ok || !got.Equal(start.Add(time.Hour)) {
		t.Errorf("long timer: %v %v", got, ok)
	}
	if _, ok := receive(t, never); ok {
		t.Error("the 48h timer fired")
	}
	if f.Pending() != 1 {
		t.Errorf("Pending = %d, want 1", f.Pending())
	}
}

func TestSetForwardFiresTimersButBackwardDoesNot(t *testing.T) {
	f := New(start)
	ch := f.After(time.Hour)
	f.Set(start.Add(-time.Hour))
	if _, ok := receive(t, ch); ok {
		t.Fatal("moving backwards fired a timer")
	}
	f.Set(start.Add(2 * time.Hour))
	if _, ok := receive(t, ch); !ok {
		t.Fatal("moving forward past the deadline did not fire")
	}
}

func TestNonPositiveDelayIsImmediate(t *testing.T) {
	f := New(start)
	for _, d := range []time.Duration{0, -time.Second} {
		got, ok := receive(t, f.After(d))
		if !ok || !got.Equal(start) {
			t.Errorf("After(%v) = %v, %v; want the current time at once", d, got, ok)
		}
	}
	if f.Pending() != 0 {
		t.Errorf("Pending = %d", f.Pending())
	}
}

func TestAdvanceNegativePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Advance(-1s) should panic")
		}
	}()
	New(start).Advance(-time.Second)
}

// The technique this clock exists for: a goroutine waits on the clock, the test releases it, and no
// real time passes.
func TestDrivingAGoroutineWithoutSleeping(t *testing.T) {
	f := New(start)
	done := make(chan time.Time, 1)
	go func() { done <- <-f.After(10 * time.Minute) }()

	deadline := time.Now().Add(2 * time.Second)
	for f.Pending() != 1 { // wait until the goroutine is really waiting
		if time.Now().After(deadline) {
			t.Fatal("goroutine never started waiting")
		}
		time.Sleep(time.Millisecond)
	}

	f.Advance(9 * time.Minute)
	select {
	case <-done:
		t.Fatal("released too early")
	case <-time.After(5 * time.Millisecond):
	}
	f.Advance(time.Minute)
	select {
	case got := <-done:
		if !got.Equal(start.Add(10 * time.Minute)) {
			t.Fatalf("got %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine was not released")
	}
}

func TestConcurrentUse(t *testing.T) {
	f := New(start)
	chans := make([]<-chan time.Time, 50)
	done := make(chan struct{})
	for i := range chans {
		go func() {
			chans[i] = f.After(time.Duration(i+1) * time.Second)
			done <- struct{}{}
		}()
	}
	for range chans {
		<-done
	}
	var advancers [10]chan struct{}
	for i := range advancers {
		advancers[i] = make(chan struct{})
		go func() {
			defer close(advancers[i])
			for range 10 {
				f.Advance(time.Second)
				f.Now()
				f.Pending()
			}
		}()
	}
	for _, c := range advancers {
		<-c
	}
	for i, ch := range chans {
		if _, ok := receive(t, ch); !ok {
			t.Fatalf("timer %d did not fire after 100s of fake time", i)
		}
	}
}
