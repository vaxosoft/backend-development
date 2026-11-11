package tagstats

import (
	"fmt"
	"slices"
	"sync"
	"testing"
)

func makeTags(n int) []string {
	tags := make([]string, n)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag-%d", i%(n/2+1)) // roughly half of them are duplicates
	}
	return tags
}

func TestDedupVariantsAgree(t *testing.T) {
	for _, n := range []int{0, 1, 2, 10, 101} {
		in := makeTags(n)
		fast, slow := Dedup(in), DedupSlow(in)
		if !slices.Equal(fast, slow) {
			t.Fatalf("n=%d: Dedup=%q DedupSlow=%q", n, fast, slow)
		}
	}
}

func TestDescribe(t *testing.T) {
	for n, want := range map[int]string{0: "no tags", 1: "one tag", 7: "a few tags"} {
		if got := Describe(n); got != want {
			t.Errorf("Describe(%d) = %q, want %q", n, got, want)
		}
	}
}

// Benchmarks are noisy; allocation counts are not, so they make good regression tests.
func TestDedupAllocations(t *testing.T) {
	in := makeTags(100)
	if allocs := testing.AllocsPerRun(100, func() { DedupSlow(in) }); allocs != 1 {
		t.Errorf("DedupSlow allocated %v times per run, want exactly 1 (the result)", allocs)
	}
	// The exact count depends on the runtime's map implementation, so assert a bound, not a number.
	if allocs := testing.AllocsPerRun(100, func() { Dedup(in) }); allocs > 10 {
		t.Errorf("Dedup allocated %v times per run, want <= 10", allocs)
	}
}

func TestCounterIsSafeForConcurrentUse(t *testing.T) {
	var c Counter
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 100 {
				c.Add("docs")
			}
		})
	}
	wg.Wait()
	if got := c.Count("docs"); got != 2000 {
		t.Fatalf("Count = %d, want 2000", got)
	}
}

func BenchmarkDedup(b *testing.B) {
	impls := []struct {
		name string
		fn   func([]string) []string
	}{{"slice", DedupSlow}, {"map", Dedup}}
	for _, n := range []int{10, 1000} {
		in := makeTags(n)
		for _, impl := range impls {
			b.Run(fmt.Sprintf("n=%d/%s", n, impl.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() { // Go 1.24+: no b.N, setup above the loop is excluded from the timing
					impl.fn(in)
				}
			})
		}
	}
}
