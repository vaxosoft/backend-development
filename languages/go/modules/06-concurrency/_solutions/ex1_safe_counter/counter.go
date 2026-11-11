// Package counter is exercise 1: a counter map that is safe for concurrent use.
package counter

import (
	"maps"
	"sync"
)

// Counter counts events by key. The zero value must be ready to use and
// every method must be safe to call from many goroutines at once.
//
// Run the tests with -race: go test -race ./modules/06-concurrency/exercises/ex1_safe_counter/
type Counter struct {
	mu     sync.Mutex
	counts map[string]int
}

// Inc adds one to key.
func (c *Counter) Inc(key string) {
	c.Add(key, 1)
}

// Add adds n to key.
func (c *Counter) Add(key string, n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[key] += n
}

// Get returns the current count for key (0 if never seen).
func (c *Counter) Get(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}

// Snapshot returns a copy of all counts. Changing the copy must not affect the Counter.
func (c *Counter) Snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		return map[string]int{}
	}
	return maps.Clone(c.counts)
}
