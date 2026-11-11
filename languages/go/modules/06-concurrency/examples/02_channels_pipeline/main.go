// Channels: ownership, closing, range, select, and a generate -> square -> merge pipeline.
package main

import (
	"fmt"
	"slices"
	"sync"
)

// generate owns its output channel: it is the only sender, so it closes it.
func generate(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, n := range nums {
			out <- n
		}
	}()
	return out
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			out <- n * n
		}
	}()
	return out
}

// merge fans several channels into one; a WaitGroup tells it when every input is drained.
func merge(ins ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	for _, in := range ins {
		wg.Go(func() {
			for v := range in {
				out <- v
			}
		})
	}
	go func() {
		wg.Wait()
		close(out) // only after every sender is done
	}()
	return out
}

func main() {
	// Unbuffered: send and receive meet. Buffered: send only blocks when full.
	buf := make(chan string, 2)
	buf <- "a"
	buf <- "b"
	fmt.Println(len(buf), cap(buf))
	fmt.Println(<-buf, <-buf)

	// Pipeline: stages are connected by channels and run concurrently.
	var got []int
	for v := range square(generate(1, 2, 3, 4)) {
		got = append(got, v)
	}
	fmt.Println("pipeline:", got)

	// Fan-out to two squarers, fan-in with merge. Arrival order is not deterministic, so sort.
	src := generate(1, 2, 3, 4, 5, 6)
	var merged []int
	for v := range merge(square(src), square(src)) {
		merged = append(merged, v)
	}
	slices.Sort(merged)
	fmt.Println("fan-out/fan-in:", merged)

	// Receiving from a closed channel yields the zero value and ok=false.
	done := make(chan struct{})
	close(done)
	_, ok := <-done
	fmt.Println("closed channel ok:", ok)

	// select picks a ready case; default makes it non-blocking.
	results := make(chan int, 1)
	select {
	case v := <-results:
		fmt.Println("got", v)
	default:
		fmt.Println("nothing ready")
	}
	results <- 42
	select {
	case v := <-results:
		fmt.Println("got", v)
	default:
		fmt.Println("nothing ready")
	}
}
