package counter

import (
	"maps"
	"sync"
	"testing"
)

func TestZeroValueIsUsable(t *testing.T) {
	var c Counter
	if got := c.Get("missing"); got != 0 {
		t.Fatalf("Get on empty counter = %d", got)
	}
	if got := len(c.Snapshot()); got != 0 {
		t.Fatalf("Snapshot on empty counter has %d entries", got)
	}
	c.Inc("a")
	c.Add("a", 4)
	if got := c.Get("a"); got != 5 {
		t.Fatalf("Get(a) = %d, want 5", got)
	}
}

func TestConcurrentUpdates(t *testing.T) {
	var c Counter
	const goroutines, each = 50, 1000
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			for range each {
				c.Inc("shared")
				if i%2 == 0 {
					c.Add("even", 2)
				}
			}
		})
	}
	wg.Wait()
	if got := c.Get("shared"); got != goroutines*each {
		t.Fatalf("shared = %d, want %d (lost updates)", got, goroutines*each)
	}
	if got := c.Get("even"); got != goroutines/2*each*2 {
		t.Fatalf("even = %d", got)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	var c Counter
	c.Inc("a")
	snap := c.Snapshot()
	snap["a"] = 100
	snap["b"] = 1
	if got := c.Get("a"); got != 1 {
		t.Fatalf("mutating the snapshot changed the counter: a = %d", got)
	}
	if got := c.Get("b"); got != 0 {
		t.Fatalf("mutating the snapshot added a key: b = %d", got)
	}
}

func TestSnapshotWhileWriting(t *testing.T) {
	var c Counter
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				c.Inc("x")
			}
		}
	})
	for range 200 {
		snap := c.Snapshot()
		_ = maps.Clone(snap) // reading the snapshot must not race with the writer
	}
	close(stop)
	wg.Wait()
}
