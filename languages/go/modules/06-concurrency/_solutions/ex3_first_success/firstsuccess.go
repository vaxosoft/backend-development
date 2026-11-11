// Package firstsuccess is exercise 3: race several attempts and keep the first success.
package firstsuccess

import (
	"context"
	"errors"
	"sync"
)

// ErrNoFunctions is returned when FirstSuccess is called with nothing to run.
var ErrNoFunctions = errors.New("no functions to run")

// FirstSuccess starts every fn concurrently and returns the first successful result.
//
//   - As soon as one succeeds, the context given to the others is cancelled.
//   - FirstSuccess returns only after every started fn has returned: no goroutine is left behind.
//   - Failures are ignored while another attempt may still succeed. If all fail, the returned
//     error wraps every individual error (errors.Join).
//   - If ctx is cancelled before any success, the returned error satisfies errors.Is(err, ctx.Err()).
//   - With no functions it returns ErrNoFunctions.
func FirstSuccess[T any](ctx context.Context, fns ...func(context.Context) (T, error)) (T, error) {
	var zero T
	if len(fns) == 0 {
		return zero, ErrNoFunctions
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		idx int
		val T
		err error
	}
	results := make(chan result, len(fns)) // buffered: a late finisher never blocks
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Go(func() {
			v, err := fn(runCtx)
			results <- result{i, v, err}
		})
	}

	errs := make([]error, len(fns))
	for range fns {
		r := <-results
		if r.err == nil {
			cancel()
			wg.Wait()
			return r.val, nil
		}
		errs[r.idx] = r.err
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	return zero, errors.Join(errs...)
}
