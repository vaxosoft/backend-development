// Goroutines, sync.WaitGroup, sync.Mutex, sync.RWMutex and sync/atomic.
package main

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// Stats is safe for concurrent use. The mutex guards the map; it must never be copied.
type Stats struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewStats() *Stats { return &Stats{counts: map[string]int{}} }

func (s *Stats) Inc(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[key]++
}

func (s *Stats) Snapshot() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.counts))
	for k, v := range s.counts {
		out[k] = v
	}
	return out
}

func main() {
	const workers, perWorker = 8, 1000

	stats := NewStats()
	var hits atomic.Int64
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() { // Go 1.25: Add(1) + go + deferred Done
			for range perWorker {
				stats.Inc([]string{"even", "odd"}[i%2])
				hits.Add(1)
			}
		})
	}
	wg.Wait()

	snap := stats.Snapshot()
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(k, snap[k])
	}
	fmt.Println("atomic total:", hits.Load())

	// Each goroutine gets its own copy of i (per-iteration loop variables since Go 1.22),
	// and writes only to its own slice element, so no lock is needed.
	squares := make([]int, 5)
	var wg2 sync.WaitGroup
	for i := range squares {
		wg2.Go(func() { squares[i] = i * i })
	}
	wg2.Wait()
	fmt.Println(squares)

	// sync.Once runs initialisation exactly once, however many goroutines race to it.
	var once sync.Once
	var inits atomic.Int32
	var wg3 sync.WaitGroup
	for range 10 {
		wg3.Go(func() { once.Do(func() { inits.Add(1) }) })
	}
	wg3.Wait()
	fmt.Println("init ran", inits.Load(), "time(s)")
}
