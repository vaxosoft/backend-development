// Package tagstats is example 04: benchmarks, allocation assertions, coverage and the race detector.
//
//	go test -v ./modules/12-testing/examples/04_benchmarks_and_coverage/
//	go test -run '^$' -bench . -benchmem ./modules/12-testing/examples/04_benchmarks_and_coverage/
//	go test -cover ./modules/12-testing/examples/04_benchmarks_and_coverage/
//	go test -race ./modules/12-testing/examples/04_benchmarks_and_coverage/
package tagstats

import (
	"slices"
	"sync"
)

// DedupSlow removes duplicates in O(n²) by scanning the output.
func DedupSlow(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// Dedup removes duplicates in O(n) with a set, keeping first-seen order.
func Dedup(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if _, dup := seen[t]; !dup {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

// Describe summarises a tag count. Its last branch is deliberately left untested so the coverage
// report has something to show.
func Describe(n int) string {
	switch {
	case n == 0:
		return "no tags"
	case n == 1:
		return "one tag"
	case n <= 10:
		return "a few tags"
	default:
		return "too many tags"
	}
}

// Counter counts tag usage and is safe for concurrent use.
type Counter struct {
	mu     sync.Mutex
	counts map[string]int
}

func (c *Counter) Add(tag string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[tag]++
}

func (c *Counter) Count(tag string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[tag]
}
