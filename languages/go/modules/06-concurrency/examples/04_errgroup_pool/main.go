// errgroup: bounded fan-out with the first error cancelling the rest, results kept in input order.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

var errBoom = errors.New("boom")

// fetch simulates a call that takes a few ms and fails for id 4.
func fetch(ctx context.Context, id int) (string, error) {
	select {
	case <-time.After(5 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if id == 4 {
		return "", fmt.Errorf("fetch %d: %w", id, errBoom)
	}
	return fmt.Sprintf("item-%d", id), nil
}

func fetchAll(ctx context.Context, ids []int, limit int, maxSeen *atomic.Int32) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	results := make([]string, len(ids)) // each goroutine writes only its own index
	var inFlight atomic.Int32
	for i, id := range ids {
		g.Go(func() error {
			n := inFlight.Add(1)
			defer inFlight.Add(-1)
			for { // record the high-water mark
				old := maxSeen.Load()
				if n <= old || maxSeen.CompareAndSwap(old, n) {
					break
				}
			}
			v, err := fetch(ctx, id)
			if err != nil {
				return err
			}
			results[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func main() {
	var maxSeen atomic.Int32
	got, err := fetchAll(context.Background(), []int{1, 2, 3, 5, 6, 7, 8, 9}, 3, &maxSeen)
	fmt.Println(got, err)
	fmt.Println("never more than 3 in flight:", maxSeen.Load() <= 3)

	got, err = fetchAll(context.Background(), []int{1, 2, 3, 4, 5, 6}, 6, &maxSeen)
	fmt.Println(got, err, errors.Is(err, errBoom))

	// Cancelling the parent stops the whole group.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = fetchAll(ctx, []int{1, 2, 3}, 2, &maxSeen)
	fmt.Println("cancelled:", err)
}
