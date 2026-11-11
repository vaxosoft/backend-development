// Package retry is exercise 2: retries with exponential backoff, only where it is safe.
package retry

import (
	"bytes"
	"context"
	"io"
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

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
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
	wait := p.Sleep
	if wait == nil {
		wait = sleep
	}
	for attempt := 0; ; attempt++ {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, r)
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(req)
		retryable := err != nil || resp.StatusCode >= 500
		if !retryable || !idempotent(method) || attempt >= p.MaxRetries {
			return resp, err
		}
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		if err := wait(ctx, p.Base<<attempt); err != nil {
			return nil, err
		}
	}
}
