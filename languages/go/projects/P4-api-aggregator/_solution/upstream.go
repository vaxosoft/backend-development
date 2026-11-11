package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Error values describing why an id could not be fetched. Their text is what
// clients see in the "errors" array.
var (
	errTimeout     = errors.New("upstream timeout")
	errUnreachable = errors.New("upstream unreachable")
	errInvalidJSON = errors.New("upstream returned invalid JSON")
)

// statusError is a non-2xx answer from the upstream.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("upstream status %d", e.code) }

func (e statusError) retryable() bool { return e.code >= 500 }

// fetch returns the JSON body of GET {UpstreamURL}/items/{id}, retrying
// transient failures with exponential backoff.
func (s *Service) fetch(ctx context.Context, id int) (json.RawMessage, error) {
	var err error
	for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			if werr := sleep(ctx, s.cfg.BackoffBase<<(attempt-1)); werr != nil {
				return nil, werr
			}
		}
		var data json.RawMessage
		data, err = s.attempt(ctx, id)
		if err == nil || !retryable(err) {
			return data, err
		}
	}
	return nil, err
}

// attempt performs one call. The per-call timeout also covers reading the body.
func (s *Service) attempt(ctx context.Context, id int) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, s.cfg.CallTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/items/%d", strings.TrimRight(s.cfg.UpstreamURL, "/"), id)
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errUnreachable
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, classify(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, statusError{code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, classify(ctx, err)
	}
	if !json.Valid(body) {
		return nil, errInvalidJSON
	}
	return json.RawMessage(body), nil
}

// classify maps a transport error to one of our error values. The parent
// context is checked first: its cancellation must not look like a retryable
// timeout of the single call.
func classify(parent context.Context, err error) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errTimeout
	}
	return errUnreachable
}

func retryable(err error) bool {
	var se statusError
	if errors.As(err, &se) {
		return se.retryable()
	}
	return errors.Is(err, errTimeout) || errors.Is(err, errUnreachable)
}

// sleep waits for d or until ctx is done, whichever comes first.
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
