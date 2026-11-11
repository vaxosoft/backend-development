// Package boundedmap is exercise 2: run a function over a slice with bounded concurrency.
package boundedmap

import "context"

// Map calls fn for every element of in, with at most limit calls running at the same time
// (a limit below 1 means 1), and returns the results in the order of in.
//
//   - The first error is returned (not a context.Canceled caused by it).
//   - After an error, the context passed to fn is cancelled and fn is not called for
//     elements that have not started yet.
//   - Map returns only after every call it started has returned: no goroutine outlives it.
//   - If ctx is already cancelled, fn is never called and ctx.Err() is returned.
//   - Empty input returns an empty, non-nil slice.
func Map[T, R any](ctx context.Context, limit int, in []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	panic("TODO")
}
