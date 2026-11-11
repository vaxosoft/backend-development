// Package retry is exercise 2: retries with exponential backoff, only where it is safe.
package retry

import (
	"context"
	"net/http"
	"time"
)

// Doer is the part of *http.Client this package needs; tests pass a fake.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Policy configures Do.
type Policy struct {
	// MaxRetries is the number of extra attempts after the first one.
	MaxRetries int
	// Base is the delay before the first retry; each further retry doubles it (Base, 2*Base, 4*Base, ...).
	Base time.Duration
	// Sleep waits for d or until ctx is done (and then returns ctx.Err()).
	// When nil, Do must use a real, context-aware wait.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Do sends method url with body (nil for none) and retries according to p.
//
// Rules:
//   - A fresh *http.Request is built for every attempt (a reader cannot be replayed).
//   - Retry only idempotent methods (GET, HEAD, PUT, DELETE, OPTIONS); never POST or PATCH.
//   - Retry on transport errors and on 5xx responses. Never retry other statuses (2xx, 3xx, 4xx).
//   - Close (after draining) the body of every 5xx response that is retried.
//   - Wait before each retry, but not after the last attempt.
//   - When the retries are used up, return the last result unchanged: the final 5xx response
//     with a nil error, or the final transport error.
//   - If the wait is interrupted by the context, return that context error immediately.
func Do(ctx context.Context, c Doer, p Policy, method, url string, body []byte) (*http.Response, error) {
	panic("TODO")
}
