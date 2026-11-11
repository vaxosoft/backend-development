// Example 02: Client.Timeout, per-request context deadlines and telling them apart.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

func main() {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release: // never happens during the demo
		case <-r.Context().Done(): // the client gave up
		}
	}))
	defer srv.Close()
	defer close(release)

	// 1. Whole-exchange timeout configured on the client.
	short := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := short.Get(srv.URL)
	fmt.Println("client timeout:", os.IsTimeout(err), errors.Is(err, context.DeadlineExceeded))

	// 2. Per-request deadline from a context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err = http.DefaultClient.Do(req)
	fmt.Println("context deadline:", errors.Is(err, context.DeadlineExceeded))

	// 3. Explicit cancellation is a different error from a timeout.
	ctx, cancel = context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err = http.DefaultClient.Do(req)
	fmt.Println("cancelled:", errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded))

	// 4. Finer-grained limits live on the Transport.
	tr := &http.Transport{
		ResponseHeaderTimeout: 2 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   8,
	}
	defer tr.CloseIdleConnections()
	fmt.Println("transport configured:", tr.ResponseHeaderTimeout)
}
