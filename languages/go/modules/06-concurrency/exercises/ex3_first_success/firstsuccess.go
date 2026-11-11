// Package firstsuccess is exercise 3: race several attempts and keep the first success.
package firstsuccess

import (
	"context"
	"errors"
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
	panic("TODO")
}
