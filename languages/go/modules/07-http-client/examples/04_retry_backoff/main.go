// Example 04: retries with exponential backoff, only where it is safe.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

func backoff(base time.Duration, attempt int) time.Duration { return base << attempt }

// doRetry rebuilds the request from body on every attempt: a consumed reader cannot be replayed.
func doRetry(ctx context.Context, c *http.Client, method, url string, body []byte, retries int, base time.Duration) (*http.Response, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(req)
		retryable := false
		switch {
		case err != nil:
			lastErr, retryable = err, true
		case resp.StatusCode >= 500:
			lastErr, retryable = fmt.Errorf("upstream status %d", resp.StatusCode), true
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		default:
			return resp, nil // success or a 4xx: retrying would not change the answer
		}
		if !retryable || !idempotent(method) || attempt == retries {
			return nil, fmt.Errorf("giving up after %d attempt(s): %w", attempt+1, lastErr)
		}
		select {
		case <-time.After(backoff(base, attempt)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func main() {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		switch {
		case r.URL.Path == "/missing":
			http.NotFound(w, r)
		case r.URL.Path == "/flaky" && n <= 2:
			http.Error(w, "try later", http.StatusServiceUnavailable)
		default:
			fmt.Fprint(w, "ok")
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := srv.Client()
	try := func(name, method, path string) {
		calls.Store(0)
		resp, err := doRetry(ctx, c, method, srv.URL+path, nil, 3, time.Millisecond)
		switch {
		case err != nil:
			fmt.Printf("%-12s calls=%d error=%v\n", name, calls.Load(), err)
		default:
			resp.Body.Close()
			fmt.Printf("%-12s calls=%d status=%d\n", name, calls.Load(), resp.StatusCode)
		}
	}

	try("flaky GET", http.MethodGet, "/flaky")
	try("missing GET", http.MethodGet, "/missing")
	try("flaky POST", http.MethodPost, "/flaky")

	fmt.Print("schedule (base 100ms):")
	for i := range 4 {
		fmt.Print(" ", backoff(100*time.Millisecond, i))
	}
	fmt.Println()

	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := doRetry(ctx, c, http.MethodGet, srv.URL+"/flaky", nil, 3, time.Hour)
	fmt.Println("cancelled context:", errors.Is(err, context.Canceled))
}
